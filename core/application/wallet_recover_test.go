package application

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/anytype-heart/pb"
	"github.com/anyproto/anytype-heart/pkg/lib/core"
	"github.com/anyproto/anytype-heart/pkg/lib/pb/model"
)

func TestWalletRecoverSessionSigningKey(t *testing.T) {
	newMnemonic := func(t *testing.T) string {
		mnemonic, err := core.WalletGenerateMnemonic(12)
		require.NoError(t, err)
		return mnemonic
	}
	recoverWallet := func(t *testing.T, s *Service, mnemonic string) {
		require.NoError(t, s.WalletRecover(&pb.RpcWalletRecoverRequest{RootPath: t.TempDir(), Mnemonic: mnemonic}))
	}
	startSession := func(t *testing.T, s *Service, mnemonic string) string {
		res, err := s.CreateSession(&pb.RpcWalletCreateSessionRequest{
			Auth: &pb.RpcWalletCreateSessionRequestAuthOfMnemonic{Mnemonic: mnemonic},
		})
		require.NoError(t, err)
		return res.Token
	}

	t.Run("recovering the loaded wallet again keeps live sessions valid", func(t *testing.T) {
		// given
		s := New()
		mnemonic := newMnemonic(t)
		recoverWallet(t, s, mnemonic)
		token := startSession(t, s, mnemonic)
		want := model.AccountAuth_Full

		// when
		recoverWallet(t, s, mnemonic)

		// then
		got, err := s.ValidateSessionToken(token)
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("recovering another wallet invalidates sessions of the previous one", func(t *testing.T) {
		// given
		s := New()
		mnemonic := newMnemonic(t)
		recoverWallet(t, s, mnemonic)
		token := startSession(t, s, mnemonic)

		// when
		recoverWallet(t, s, newMnemonic(t))

		// then
		_, err := s.ValidateSessionToken(token)
		require.Error(t, err)
	})
}
