#!/usr/bin/env node

import { existsSync, readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { createHmac, randomBytes } from 'node:crypto'

const repoRoot = resolve(new URL('..', import.meta.url).pathname)
const envPath = resolve(repoRoot, 'deploy/.env.localdev')

const envFile = loadEnvFile(envPath)
const env = new Proxy(process.env, {
  get(target, key) {
    return target[key] ?? envFile[key]
  },
})

const baseURL = trimTrailingSlash(
  env.SUB2API_BASE_URL
    ?? env.TOUCH_PIE_BASE_URL
    ?? `http://${env.BIND_HOST ?? '127.0.0.1'}:${env.SERVER_PORT ?? '8081'}`
)
const email = env.TOUCH_PIE_SMOKE_EMAIL ?? env.SUB2API_EMAIL ?? env.ADMIN_EMAIL
const password = env.TOUCH_PIE_SMOKE_PASSWORD ?? env.SUB2API_PASSWORD ?? env.ADMIN_PASSWORD
const apiKeyID = parseOptionalInt(env.TOUCH_PIE_SMOKE_API_KEY_ID)
const timeoutMs = parseOptionalInt(env.TOUCH_PIE_SMOKE_TIMEOUT_MS) ?? 10000
const smokeGroupName = env.TOUCH_PIE_SMOKE_GROUP_NAME ?? 'Touch Pie Smoke OpenAI'

const checkpoints = []

main().catch((error) => {
  fail(error)
})

async function main() {
  if (!email || !password) {
    throw new SmokeError(
      'config',
      '缺少本地登录账号：请设置 TOUCH_PIE_SMOKE_EMAIL/TOUCH_PIE_SMOKE_PASSWORD，或在 deploy/.env.localdev 中配置 ADMIN_EMAIL/ADMIN_PASSWORD'
    )
  }

  console.log(`Touch Pie localdev smoke flow: ${baseURL}`)

  await step('health', '检查后端 /health', async () => {
    const raw = await request('GET', '/health', { unwrap: false })
    if (raw?.status !== 'ok') {
      throw new Error(`unexpected health payload: ${JSON.stringify(raw)}`)
    }
  })

  const login = await step('login', `登录本地用户 ${email}`, async () => {
    const data = await request('POST', '/api/v1/auth/login', {
      body: { email, password, turnstile_token: '' },
    })
    if (data?.requires_2fa) {
      throw new Error('当前账号启用了 2FA，请换一个本地 smoke 账号或先关闭 2FA')
    }
    assertString(data?.access_token, 'login.access_token')
    return data
  })

  const userToken = login.access_token

  const key = await step('api-key', '通过 Touch Pie bootstrap 准备 TouchX API key', async () => {
    if (apiKeyID) {
      return { id: apiKeyID, key: undefined, source: 'env' }
    }

    const bootstrap = await request('GET', '/api/v1/touch-pie/bootstrap', {
      token: userToken,
    })
    if (bootstrap?.provider_name !== 'TouchX') {
      throw new Error(`unexpected provider_name: ${bootstrap?.provider_name}`)
    }
    if (bootstrap?.default_model !== 'gpt-5.5') {
      throw new Error(`unexpected default_model: ${bootstrap?.default_model}`)
    }
    const group = bootstrap?.groups?.[0] ?? await ensureSmokeGroup(userToken)
    const existing = bootstrap?.api_keys?.find?.((candidate) => candidate?.id && candidate?.status === 'active')
      ?? bootstrap?.api_keys?.find?.((candidate) => candidate?.id)
    if (existing?.id) {
      return { id: existing.id, key: undefined, source: 'reused' }
    }

    const created = await request('POST', '/api/v1/touch-pie/api-keys', {
      token: userToken,
      body: {
        name: 'TouchX',
        group_id: group.id,
      },
    })
    if (!created?.id) {
      throw new Error(`API key create returned no id: ${JSON.stringify(redact(created))}`)
    }
    if (created?.provider_name !== 'TouchX') {
      throw new Error(`created key missing TouchX metadata: ${JSON.stringify(redact(created))}`)
    }
    return { id: created.id, key: created.key, source: 'created' }
  })

  const device = await step('device-start', '发起 Touch Pie device start', async () => {
    const data = await request('POST', '/api/v1/touch-pie/device/start', {
      body: { base_url: baseURL },
    })
    assertString(data?.device_code, 'device_start.device_code')
    assertString(data?.user_code, 'device_start.user_code')
    assertString(data?.verification_uri, 'device_start.verification_uri')
    return data
  })

  await step('token-before-approve', '确认未授权 token 仍为 pending', async () => {
    const result = await requestMaybeError('POST', '/api/v1/touch-pie/device/token', {
      body: { device_code: device.device_code },
    })
    if (result.ok) {
      throw new Error('device token 在 approve 前不应成功')
    }
    if (result.status !== 400 || result.payload?.reason !== 'TOUCH_PIE_AUTHORIZATION_PENDING') {
      throw new Error(`expected pending error, got status=${result.status} body=${JSON.stringify(redact(result.payload))}`)
    }
  })

  await step('approve', '使用登录用户 approve user_code', async () => {
    const data = await request('POST', '/api/v1/touch-pie/device/approve', {
      token: userToken,
      body: { user_code: device.user_code, api_key_id: key.id },
    })
    if (data?.approved !== true) {
      throw new Error(`approve returned unexpected payload: ${JSON.stringify(data)}`)
    }
  })

  const deviceToken = await step('token-after-approve', '用 device_code 换取 Touch Pie token', async () => {
    const data = await request('POST', '/api/v1/touch-pie/device/token', {
      body: { device_code: device.device_code },
    })
    assertString(data?.access_token, 'device_token.access_token')
    assertString(data?.refresh_token, 'device_token.refresh_token')
    if (data?.token_type !== 'Bearer') {
      throw new Error(`unexpected token_type: ${data?.token_type}`)
    }
    if (Number(data?.api_key_id) !== Number(key.id)) {
      throw new Error(`device token api_key_id mismatch: got ${data?.api_key_id}, want ${key.id}`)
    }
    return data
  })

  await step('token-consumed', '确认 device_code 已消费且不能重复换 token', async () => {
    const result = await requestMaybeError('POST', '/api/v1/touch-pie/device/token', {
      body: { device_code: device.device_code },
    })
    if (result.ok) {
      throw new Error('device token 不应允许重复消费')
    }
    if (result.status !== 400 || result.payload?.reason !== 'TOUCH_PIE_DEVICE_CONSUMED') {
      throw new Error(`expected consumed error, got status=${result.status} body=${JSON.stringify(redact(result.payload))}`)
    }
  })

  const exported = await step('export-api-key', `使用 Touch Pie token 导出 API key #${key.id}`, async () => {
    const data = await request('POST', `/api/v1/touch-pie/api-keys/${key.id}/export`, {
      token: deviceToken.access_token,
      body: {},
    })
    if (Number(data?.id) !== Number(key.id)) {
      throw new Error(`exported id mismatch: got ${data?.id}, want ${key.id}`)
    }
    assertString(data?.key, 'export.key')
    if (key.key && data.key !== key.key) {
      throw new Error('exported key 与用户 API key 不一致')
    }
    if (data?.provider_name !== 'TouchX' || data?.default_model !== 'gpt-5.5') {
      throw new Error(`export missing TouchX metadata: ${JSON.stringify(redact(data))}`)
    }
    return data
  })

  await step('models-fast-lane', '使用导出的 API key 签名验证 Touch Pie fast lane', async () => {
    const signed = await requestMaybeError('GET', '/v1/models', {
      token: exported.key,
      headers: {
        'x-touch-pie': signTouchPieHeader(exported.key),
      },
    })
    if (!signed.ok) {
      throw new Error(`HTTP ${signed.status} GET /v1/models: ${JSON.stringify(redact(signed.payload))}`)
    }
    const provider = signed.headers?.['x-sub2api-provider']
    const providerSource = signed.headers?.['x-sub2api-provider-source']
    if (provider !== 'TouchX' || providerSource !== 'touchx') {
      throw new Error(`missing TouchX provider headers: ${JSON.stringify(signed.headers)}`)
    }
    const models = Array.isArray(signed.payload?.data) ? signed.payload.data : []
    if (!models.some((model) => model?.id === 'gpt-5.5' && model?.owned_by === 'TouchX')) {
      throw new Error(`TouchX model metadata missing from /v1/models: ${JSON.stringify(redact(signed.payload))}`)
    }
  })

  console.log('')
  console.log('Touch Pie smoke flow passed.')
  console.log(`- API key id: ${exported.id} (${key.source})`)
  console.log(`- Exported key: ${mask(exported.key)}`)
}

async function step(name, label, fn) {
  const started = Date.now()
  try {
    const result = await fn()
    checkpoints.push({ name, ok: true, ms: Date.now() - started })
    console.log(`✓ ${label}`)
    return result
  } catch (cause) {
    checkpoints.push({ name, ok: false, ms: Date.now() - started })
    throw new SmokeError(name, explainFailure(name, cause), cause)
  }
}

async function ensureSmokeGroup(token) {
  let result = await requestMaybeError('POST', '/api/v1/admin/groups', {
    token,
    body: {
      name: smokeGroupName,
      description: 'Created by Touch Pie localdev smoke.',
      platform: 'openai',
      rate_multiplier: 1,
      subscription_type: 'standard',
      is_exclusive: false,
    },
  })
  if (!result.ok && result.status === 423 && result.payload?.code === 'ADMIN_COMPLIANCE_ACK_REQUIRED') {
    await acceptAdminCompliance(token)
    result = await requestMaybeError('POST', '/api/v1/admin/groups', {
      token,
      body: {
        name: smokeGroupName,
        description: 'Created by Touch Pie localdev smoke.',
        platform: 'openai',
        rate_multiplier: 1,
        subscription_type: 'standard',
        is_exclusive: false,
      },
    })
  }
  if (!result.ok) {
    throw new Error(`HTTP ${result.status} POST /api/v1/admin/groups: ${JSON.stringify(redact(result.payload))}`)
  }
  const created = result.payload
  if (!created?.id) {
    throw new Error(`failed to create smoke group: ${JSON.stringify(redact(created))}`)
  }
  return created
}

async function acceptAdminCompliance(token) {
  const status = await request('GET', '/api/v1/admin/compliance', { token })
  if (status?.required === false) {
    return
  }
  const phrase = status?.ack_phrase_zh || '我已阅读、理解并同意 Sub2API 部署与运营合规承诺'
  await request('POST', '/api/v1/admin/compliance/accept', {
    token,
    body: {
      phrase,
      language: 'zh',
    },
  })
}

async function request(method, path, options = {}) {
  const result = await requestMaybeError(method, path, options)
  if (!result.ok) {
    const body = JSON.stringify(redact(result.payload))
    throw new Error(`HTTP ${result.status} ${method} ${path}: ${body}`)
  }
  return result.payload
}

async function requestMaybeError(method, path, options = {}) {
  const controller = new AbortController()
  const timer = setTimeout(() => controller.abort(), timeoutMs)
  try {
    const headers = { Accept: 'application/json' }
    let body
    if (options.body !== undefined) {
      headers['Content-Type'] = 'application/json'
      body = JSON.stringify(options.body)
    }
    if (options.token) {
      headers.Authorization = `Bearer ${options.token}`
    }
    Object.assign(headers, options.headers || {})

    const response = await fetch(`${baseURL}${path}`, {
      method,
      headers,
      body,
      signal: controller.signal,
    })
    const text = await response.text()
    const parsed = text ? parseJSON(text) : null
    const responseHeaders = Object.fromEntries(response.headers.entries())

    if (!response.ok) {
      return { ok: false, status: response.status, payload: parsed ?? text, headers: responseHeaders }
    }
    if (options.unwrap === false) {
      return { ok: true, status: response.status, payload: parsed, headers: responseHeaders }
    }
    if (parsed && typeof parsed === 'object' && 'code' in parsed) {
      if (parsed.code !== 0) {
        return { ok: false, status: response.status, payload: parsed, headers: responseHeaders }
      }
      return { ok: true, status: response.status, payload: parsed.data, headers: responseHeaders }
    }
    return { ok: true, status: response.status, payload: parsed, headers: responseHeaders }
  } catch (error) {
    if (error?.name === 'AbortError') {
      throw new Error(`request timed out after ${timeoutMs}ms`)
    }
    if (error?.cause) {
      const cause = error.cause
      const address = [cause.address, cause.port].filter(Boolean).join(':')
      const suffix = [cause.code, address].filter(Boolean).join(' ')
      throw new Error(`${error.message}${suffix ? ` (${suffix})` : ''}`)
    }
    throw error
  } finally {
    clearTimeout(timer)
  }
}

function explainFailure(name, cause) {
  const detail = cause?.message ?? String(cause)
  const hints = {
    config: '本地 smoke 配置不完整。',
    health: '后端不可用或端口不对。先确认 localdev 后端已启动，并检查 SUB2API_BASE_URL / SERVER_PORT。',
    login: '账号登录失败。优先检查 deploy/.env.localdev 的 ADMIN_EMAIL/ADMIN_PASSWORD、后端自动初始化日志，以及 Redis 限流是否可用。',
    'api-key': 'API key 准备失败。优先检查当前用户是否有创建/读取 key 权限、默认分组配置和数据库迁移状态。',
    'device-start': 'device start 失败。优先检查 touch_pie_device_sessions 表是否已由 154 migration 创建。',
    'token-before-approve': 'pending 状态校验失败。可能是 token 接口状态机或错误 reason 变了。',
    approve: 'approve 失败。优先检查 JWT 中间件、BackendModeUserGuard、user_code 是否被正确保存/哈希。',
    'token-after-approve': 'approve 后换 token 失败。优先检查 touch_pie_device_sessions 状态、user_id、AuthService.GenerateTokenPair。',
    'token-consumed': 'consume 状态校验失败。可能是 token 接口没有正确消费 device session。',
    'export-api-key': 'API key 导出失败。优先检查 Touch Pie token 是否能通过 JWT 中间件、key 所属 user_id 是否一致、ExportAPIKey 权限判断。',
    'models-fast-lane': 'Touch Pie fast lane 验证失败。优先检查 x-touch-pie 签名算法、API Key 分组/余额/订阅状态，以及 /v1/models 的 TouchX metadata 标记。',
  }
  return `${hints[name] ?? 'smoke flow 失败。'}\n${detail}`
}

function fail(error) {
  console.error('')
  console.error('Touch Pie smoke flow failed.')
  if (error instanceof SmokeError) {
    console.error(`卡住环节: ${error.stage}`)
    console.error(error.message)
  } else {
    console.error(error?.message ?? String(error))
  }
  if (checkpoints.length > 0) {
    console.error('')
    console.error('已执行检查点:')
    for (const point of checkpoints) {
      const mark = point.ok ? '✓' : '✗'
      console.error(`- ${mark} ${point.name} (${point.ms}ms)`)
    }
  }
  process.exitCode = 1
}

class SmokeError extends Error {
  constructor(stage, message, cause) {
    super(message)
    this.name = 'SmokeError'
    this.stage = stage
    this.cause = cause
  }
}

function loadEnvFile(path) {
  if (!existsSync(path)) {
    return {}
  }
  const entries = {}
  const content = readFileSync(path, 'utf8')
  for (const line of content.split(/\r?\n/)) {
    const trimmed = line.trim()
    if (!trimmed || trimmed.startsWith('#') || !trimmed.includes('=')) {
      continue
    }
    const index = trimmed.indexOf('=')
    const key = trimmed.slice(0, index).trim()
    const value = trimmed.slice(index + 1).trim().replace(/^['"]|['"]$/g, '')
    entries[key] = value
  }
  return entries
}

function parseJSON(text) {
  try {
    return JSON.parse(text)
  } catch {
    return text
  }
}

function assertString(value, name) {
  if (typeof value !== 'string' || value.trim() === '') {
    throw new Error(`${name} is missing`)
  }
}

function parseOptionalInt(value) {
  if (value === undefined || value === '') {
    return undefined
  }
  const parsed = Number.parseInt(value, 10)
  return Number.isFinite(parsed) ? parsed : undefined
}

function trimTrailingSlash(value) {
  return String(value).replace(/\/+$/, '')
}

function signTouchPieHeader(apiKey) {
  const ts = Math.floor(Date.now() / 1000)
  const nonce = randomBytes(12).toString('hex')
  const payload = `touch-pie:v1:${ts}:${nonce}`
  const sig = createHmac('sha256', apiKey).update(payload).digest('base64url')
  return `v1.${ts}.${nonce}.${sig}`
}

function mask(value) {
  if (!value || value.length <= 12) {
    return '***'
  }
  return `${value.slice(0, 6)}...${value.slice(-6)}`
}

function redact(value) {
  if (Array.isArray(value)) {
    return value.map(redact)
  }
  if (!value || typeof value !== 'object') {
    return value
  }
  return Object.fromEntries(Object.entries(value).map(([key, item]) => {
    if (/token|password|secret|key/i.test(key) && typeof item === 'string') {
      return [key, mask(item)]
    }
    return [key, redact(item)]
  }))
}
