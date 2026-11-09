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

// setRecoveredKeys stores the derived keys and picks the session signing key.
// Recovering the wallet that is already loaded keeps the current signing key:
// a client recovers again while other clients still hold sessions (every
// desktop tab logs in on its own after the machine wakes), and a new key would
// fail the signature check of every live token. A different wallet gets a new
// key, so no token of the previous wallet stays valid.
// The caller must hold s.lock.
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
