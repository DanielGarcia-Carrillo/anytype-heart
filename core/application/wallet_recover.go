package application

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"

	"github.com/anyproto/any-sync/util/crypto"

	"github.com/anyproto/anytype-heart/pb"
	"github.com/anyproto/anytype-heart/pkg/lib/core"
)

func (s *Service) WalletRecover(req *pb.RpcWalletRecoverRequest) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	// Validate that only one auth method is provided
	if req.Mnemonic != "" && req.AccountKey != "" {
		return errors.Join(ErrBadInput, fmt.Errorf("cannot provide both mnemonic and accountKey"))
	}

	if req.Mnemonic == "" && req.AccountKey == "" {
		return errors.Join(ErrBadInput, fmt.Errorf("either mnemonic or accountKey must be provided"))
	}

	err := os.MkdirAll(req.RootPath, 0700)
	if err != nil {
		return errors.Join(ErrFailedToCreateLocalRepo, err)
	}

	// Handle accountKey recovery
	if req.AccountKey != "" {
		// Derive keys from the provided account key
		derivationResult, err := core.WalletDeriveFromAccountMasterNode(req.AccountKey)
		if err != nil {
			return errors.Join(ErrBadInput, fmt.Errorf("invalid account key: %w", err))
		}

		if err := s.setRecoveredKeys(derivationResult); err != nil {
			return fmt.Errorf("set recovered keys: %w", err)
		}

		s.rootPath = req.RootPath
		s.fulltextPrimaryLanguage = req.FulltextPrimaryLanguage
		return nil
	}

	// Handle mnemonic recovery
	// Derive keys from mnemonic
	derivationResult, err := core.WalletAccountAt(req.Mnemonic, 0)
	if err != nil {
		return errors.Join(ErrBadInput, err)
	}

	if err := s.setRecoveredKeys(derivationResult); err != nil {
		return fmt.Errorf("set recovered keys: %w", err)
	}

	s.rootPath = req.RootPath
	s.fulltextPrimaryLanguage = req.FulltextPrimaryLanguage
	return nil
}

// setRecoveredKeys stores the derived keys. It keeps the session signing key
// when the same wallet is recovered again, so live tokens stay valid, and
// generates a new one for a different wallet. The caller must hold s.lock.
func (s *Service) setRecoveredKeys(derived crypto.DerivationResult) error {
	sameWallet := s.derivedKeys != nil &&
		len(s.sessionSigningKey) > 0 &&
		s.derivedKeys.Identity.GetPublic().Account() == derived.Identity.GetPublic().Account()

	s.derivedKeys = &derived

	if sameWallet {
		return nil
	}

	buf := make([]byte, 64)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Errorf("generate session signing key: %w", err)
	}
	s.sessionSigningKey = buf
	return nil
}
