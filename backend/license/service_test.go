package license_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"go-stock/backend/license"

	"github.com/stretchr/testify/require"
)

func TestOfflineLicense_Valid(t *testing.T) {
	secret := "dev-secret"
	now := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	lic := license.IssueOfflineLicense(
		"lic-1", "local-device", license.TypePro,
		now.Add(-time.Hour), now.Add(30*24*time.Hour), 7, secret,
	)
	v := license.NewLocalValidator(secret)
	res := v.Validate(lic, now)
	require.Equal(t, license.StatusValid, res.Status)
	require.Equal(t, license.TypePro, res.EffectiveType)
	require.True(t, res.PaperTradingAllowed)
	require.True(t, license.PaperTradingAllowed(res))
	require.Equal(t, license.StatusValid, res.License.Status)
}

func TestOfflineLicense_ExpiredFallsBackFree_PaperStillAllowed(t *testing.T) {
	now := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	lic := license.IssueOfflineLicense(
		"lic-2", "local-device", license.TypeEnterprise,
		now.Add(-48*time.Hour), now.Add(-time.Hour), 0, "",
	)
	res := license.NewLocalValidator("").Validate(lic, now)
	require.Equal(t, license.StatusExpired, res.Status)
	require.Equal(t, license.TypeFree, res.EffectiveType)
	require.True(t, res.PaperTradingAllowed, "offline/expired must not block paper trading")
}

func TestOfflineLicense_GraceKeepsType(t *testing.T) {
	now := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	lic := license.IssueOfflineLicense(
		"lic-3", "u1", license.TypePro,
		now.Add(-10*24*time.Hour), now.Add(-time.Hour), 3, "",
	)
	res := license.NewLocalValidator("").Validate(lic, now)
	require.Equal(t, license.StatusGrace, res.Status)
	require.Equal(t, license.TypePro, res.EffectiveType)
	require.True(t, res.PaperTradingAllowed)
}

func TestMissingLicense_FreeAndPaperOK(t *testing.T) {
	res := license.NewLocalValidator("").Validate(nil, time.Now().UTC())
	require.Equal(t, license.StatusMissing, res.Status)
	require.Equal(t, license.TypeFree, res.EffectiveType)
	require.True(t, res.PaperTradingAllowed)
}

func TestChecksumMismatch_Invalid(t *testing.T) {
	secret := "dev-secret"
	now := time.Now().UTC()
	lic := license.IssueOfflineLicense("lic-4", "u1", license.TypePro, now.Add(-time.Hour), time.Time{}, 0, secret)
	lic.Signature = "deadbeef"
	res := license.NewLocalValidator(secret).Validate(lic, now)
	require.Equal(t, license.StatusInvalid, res.Status)
	require.True(t, res.PaperTradingAllowed)
}

func TestOnlineFailure_DoesNotInvalidateOfflineValid(t *testing.T) {
	now := time.Now().UTC()
	lic := license.IssueOfflineLicense("lic-5", "u1", license.TypePro, now.Add(-time.Hour), now.Add(24*time.Hour), 0, "")
	lic.Mode = license.ModeOnline

	failing := onlineStub{err: errors.New("network down")}
	v := license.NewCompositeValidator(license.NewLocalValidator(""), failing)
	res := v.Validate(lic, now)
	require.Equal(t, license.StatusValid, res.Status)
	require.True(t, res.OnlineAttempted)
	require.False(t, res.OnlineOK)
	require.Equal(t, license.TypePro, res.EffectiveType)
	require.True(t, res.PaperTradingAllowed)
}

func TestService_InstallAndValidate(t *testing.T) {
	svc := license.NewService(license.NewMemoryStore(), license.NewLocalValidator(""))
	ctx := context.Background()
	res, err := svc.Validate(ctx)
	require.NoError(t, err)
	require.Equal(t, license.StatusMissing, res.Status)
	require.True(t, res.PaperTradingAllowed)

	lic := license.IssueOfflineLicense("lic-6", "local-device", license.TypeEnterprise, time.Now().Add(-time.Hour), time.Time{}, 0, "")
	require.NoError(t, svc.InstallLicense(ctx, lic))
	res, err = svc.Validate(ctx)
	require.NoError(t, err)
	require.Equal(t, license.StatusValid, res.Status)
	require.Equal(t, license.TypeEnterprise, res.EffectiveType)

	got, err := svc.GetLicense(ctx)
	require.NoError(t, err)
	require.Equal(t, license.StatusValid, got.Status)
	require.Equal(t, "lic-6", got.ID)

	require.NoError(t, svc.ClearLicense(ctx))
	res, err = svc.Validate(ctx)
	require.NoError(t, err)
	require.Equal(t, license.StatusMissing, res.Status)
	require.True(t, res.PaperTradingAllowed)
}

func TestValidateLicenseKey_Pro(t *testing.T) {
	now := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	kv := license.ValidateLicenseKey("GSTK-PRO-20271231-DEMO1234", now)
	require.True(t, kv.OK)
	require.NotNil(t, kv.License)
	require.Equal(t, license.TypePro, kv.License.Type)
	require.Equal(t, license.ModeOffline, kv.License.Mode)
	require.False(t, kv.License.ExpireAt.IsZero())
}

func TestValidateLicenseKey_Expired(t *testing.T) {
	now := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	kv := license.ValidateLicenseKey("GSTK-PRO-20200101-DEMO1234", now)
	require.False(t, kv.OK)
}

func TestValidateLicenseKey_BadFormat(t *testing.T) {
	kv := license.ValidateLicenseKey("NOT-A-KEY", time.Now().UTC())
	require.False(t, kv.OK)
}

func TestService_ActivateWithKey(t *testing.T) {
	svc := license.NewService(license.NewMemoryStore(), license.NewLocalValidator(""))
	ctx := context.Background()
	kv, err := svc.ActivateWithKey(ctx, "GSTK-ENTERPRISE-NEVER-ORG0001")
	require.NoError(t, err)
	require.True(t, kv.OK)
	require.Equal(t, license.TypeEnterprise, kv.License.Type)

	res, err := svc.Validate(ctx)
	require.NoError(t, err)
	require.Equal(t, license.StatusValid, res.Status)
	require.Equal(t, license.TypeEnterprise, res.EffectiveType)
}

type onlineStub struct{ err error }

func (o onlineStub) Check(_ *license.License, _ time.Time) error { return o.err }
