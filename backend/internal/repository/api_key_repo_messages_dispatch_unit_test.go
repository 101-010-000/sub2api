package repository

import (
	"context"
	"database/sql"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

func TestGroupEntityToService_PreservesMessagesDispatchModelConfig(t *testing.T) {
	group := &dbent.Group{
		ID:                    1,
		Name:                  "openai-dispatch",
		Platform:              service.PlatformOpenAI,
		Status:                service.StatusActive,
		SubscriptionType:      service.SubscriptionTypeStandard,
		RateMultiplier:        1,
		AllowMessagesDispatch: true,
		DefaultMappedModel:    "gpt-5.4",
		MessagesDispatchModelConfig: service.OpenAIMessagesDispatchModelConfig{
			OpusMappedModel:   "gpt-5.4-nano",
			SonnetMappedModel: "gpt-5.3-codex",
			HaikuMappedModel:  "gpt-5.4-mini",
			ExactModelMappings: map[string]string{
				"claude-sonnet-4.5": "gpt-5.4-nano",
			},
		},
	}

	got := groupEntityToService(group)
	require.NotNil(t, got)
	require.Equal(t, group.MessagesDispatchModelConfig, got.MessagesDispatchModelConfig)
}

func TestAPIKeyRepository_GetByKeyForAuth_PreservesMessagesDispatchModelConfig_SQLite(t *testing.T) {
	repo, client := newAPIKeyRepoSQLite(t)
	ctx := context.Background()
	user := mustCreateAPIKeyRepoUser(t, ctx, client, "getbykey-auth-dispatch-unit@test.com")

	group, err := client.Group.Create().
		SetName("g-auth-dispatch-unit").
		SetPlatform(service.PlatformOpenAI).
		SetStatus(service.StatusActive).
		SetSubscriptionType(service.SubscriptionTypeStandard).
		SetRateMultiplier(1).
		SetAllowMessagesDispatch(true).
		SetDefaultMappedModel("gpt-5.4").
		SetMessagesDispatchModelConfig(service.OpenAIMessagesDispatchModelConfig{
			OpusMappedModel:   "gpt-5.4-nano",
			SonnetMappedModel: "gpt-5.3-codex",
			HaikuMappedModel:  "gpt-5.4-mini",
			ExactModelMappings: map[string]string{
				"claude-sonnet-4.5": "gpt-5.4-nano",
			},
		}).
		Save(ctx)
	require.NoError(t, err)

	key := &service.APIKey{
		UserID:  user.ID,
		Key:     "sk-getbykey-auth-dispatch-unit",
		Name:    "Dispatch Key Unit",
		GroupID: &group.ID,
		Status:  service.StatusActive,
	}
	require.NoError(t, repo.Create(ctx, key))

	got, err := repo.GetByKeyForAuth(ctx, key.Key)
	require.NoError(t, err)
	require.Equal(t, key.Name, got.Name)
	require.NotNil(t, got.Group)
	require.Equal(t, group.MessagesDispatchModelConfig, got.Group.MessagesDispatchModelConfig)
}

func TestAPIKeyRepository_GetByKeyForAuth_HydratesSpeedAndSuisuSettings_SQLite(t *testing.T) {
	repo, client, db := newAPIKeyRepoSQLiteWithSpeedColumns(t)
	ctx := context.Background()
	user := mustCreateAPIKeyRepoUser(t, ctx, client, "getbykey-auth-speed-unit@test.com")

	group, err := client.Group.Create().
		SetName("g-auth-speed-unit").
		SetPlatform(service.PlatformOpenAI).
		SetStatus(service.StatusActive).
		SetSubscriptionType(service.SubscriptionTypeSubscription).
		SetRateMultiplier(1).
		Save(ctx)
	require.NoError(t, err)

	fallbackGroupID := int64(77)
	_, err = db.ExecContext(ctx, `
		UPDATE groups SET
			speed_config_enabled = TRUE,
			user_speed_config_allowed = TRUE,
			default_fast_quota_ratio = 0.2500,
			min_fast_quota_ratio = 0.1000,
			max_fast_quota_ratio = 0.8000,
			default_slow_delay_min_seconds = 2,
			default_slow_delay_max_seconds = 5,
			max_slow_delay_seconds = 8,
			default_slow_reject_rate = 0.2000,
			max_slow_reject_rate = 0.6000,
			speed_slow_reject_message = 'custom slow reject',
			suisu_enabled = TRUE,
			suisu_fallback_group_id = $1,
			suisu_slow_route_ratio = 0.3000,
			suisu_busy_route_ratio = 0.4000
		WHERE id = $2
	`, fallbackGroupID, group.ID)
	require.NoError(t, err)

	key := &service.APIKey{
		UserID:  user.ID,
		Key:     "sk-getbykey-auth-speed-unit",
		Name:    "Speed Key Unit",
		GroupID: &group.ID,
		Status:  service.StatusActive,
	}
	require.NoError(t, repo.Create(ctx, key))

	got, err := repo.GetByKeyForAuth(ctx, key.Key)
	require.NoError(t, err)
	require.NotNil(t, got.Group)
	require.True(t, got.Group.SpeedConfigEnabled)
	require.True(t, got.Group.UserSpeedConfigAllowed)
	require.InDelta(t, 0.25, got.Group.DefaultFastQuotaRatio, 0.0001)
	require.InDelta(t, 0.10, got.Group.MinFastQuotaRatio, 0.0001)
	require.InDelta(t, 0.80, got.Group.MaxFastQuotaRatio, 0.0001)
	require.Equal(t, 2, got.Group.DefaultSlowDelayMinSeconds)
	require.Equal(t, 5, got.Group.DefaultSlowDelayMaxSeconds)
	require.Equal(t, 8, got.Group.MaxSlowDelaySeconds)
	require.InDelta(t, 0.20, got.Group.DefaultSlowRejectRate, 0.0001)
	require.InDelta(t, 0.60, got.Group.MaxSlowRejectRate, 0.0001)
	require.Equal(t, "custom slow reject", got.Group.SpeedSlowRejectMessage)
	require.True(t, got.Group.SuisuEnabled)
	require.NotNil(t, got.Group.SuisuFallbackGroupID)
	require.Equal(t, fallbackGroupID, *got.Group.SuisuFallbackGroupID)
	require.InDelta(t, 0.30, got.Group.SuisuSlowRouteRatio, 0.0001)
	require.InDelta(t, 0.40, got.Group.SuisuBusyRouteRatio, 0.0001)
}

func newAPIKeyRepoSQLiteWithSpeedColumns(t *testing.T) (*apiKeyRepository, *dbent.Client, *sql.DB) {
	t.Helper()

	db, err := sql.Open("sqlite", "file:api_key_repo_speed?mode=memory&cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })

	_, err = db.Exec(`
		ALTER TABLE groups ADD COLUMN speed_config_enabled BOOLEAN NOT NULL DEFAULT FALSE;
		ALTER TABLE groups ADD COLUMN user_speed_config_allowed BOOLEAN NOT NULL DEFAULT FALSE;
		ALTER TABLE groups ADD COLUMN default_fast_quota_ratio REAL NOT NULL DEFAULT 0.3000;
		ALTER TABLE groups ADD COLUMN min_fast_quota_ratio REAL NOT NULL DEFAULT 0.1000;
		ALTER TABLE groups ADD COLUMN max_fast_quota_ratio REAL NOT NULL DEFAULT 1.0000;
		ALTER TABLE groups ADD COLUMN default_slow_delay_min_seconds INTEGER NOT NULL DEFAULT 1;
		ALTER TABLE groups ADD COLUMN default_slow_delay_max_seconds INTEGER NOT NULL DEFAULT 3;
		ALTER TABLE groups ADD COLUMN max_slow_delay_seconds INTEGER NOT NULL DEFAULT 10;
		ALTER TABLE groups ADD COLUMN default_slow_reject_rate REAL NOT NULL DEFAULT 0.0000;
		ALTER TABLE groups ADD COLUMN max_slow_reject_rate REAL NOT NULL DEFAULT 0.5000;
		ALTER TABLE groups ADD COLUMN speed_slow_reject_message TEXT NOT NULL DEFAULT 'You''ve sent too many requests.';
		ALTER TABLE groups ADD COLUMN suisu_enabled BOOLEAN NOT NULL DEFAULT FALSE;
		ALTER TABLE groups ADD COLUMN suisu_fallback_group_id INTEGER;
		ALTER TABLE groups ADD COLUMN suisu_slow_route_ratio REAL NOT NULL DEFAULT 0.0000;
		ALTER TABLE groups ADD COLUMN suisu_busy_route_ratio REAL NOT NULL DEFAULT 0.0000;
	`)
	require.NoError(t, err)

	return newAPIKeyRepositoryWithSQL(client, db), client, db
}
