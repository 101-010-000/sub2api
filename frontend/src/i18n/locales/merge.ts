type LocaleMessages = Record<string, unknown>

function isLocaleMessages(value: unknown): value is LocaleMessages {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
}

function mergeInto(target: LocaleMessages, source: LocaleMessages): void {
  for (const [key, value] of Object.entries(source)) {
    if (isLocaleMessages(value) && isLocaleMessages(target[key])) {
      mergeInto(target[key], value)
      continue
    }
    target[key] = value
  }
}

export function mergeLocaleMessages(...sources: LocaleMessages[]): LocaleMessages {
  const result: LocaleMessages = {}
  for (const source of sources) mergeInto(result, source)
  return result
}
