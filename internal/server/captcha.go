package server

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"
)

const captchaTTL = 5 * time.Minute

type captchaChallenge struct {
	Answer    string
	ExpiresAt time.Time
}

type captchaStore struct {
	mu         sync.Mutex
	challenges map[string]captchaChallenge
}

func (s *Server) loginCaptchaEnabled() bool {
	return s.store.SettingValue("loginCaptcha", "disabled") == "enabled"
}

func (s *Server) newCaptchaChallenge(now time.Time) (string, string, error) {
	left, err := secureInt(40)
	if err != nil {
		return "", "", err
	}
	right, err := secureInt(40)
	if err != nil {
		return "", "", err
	}
	left += 10
	right += 10
	answer := fmt.Sprintf("%d", left+right)
	id, err := s.captchas.create(answer, now.Add(captchaTTL), now)
	if err != nil {
		return "", "", err
	}
	return id, fmt.Sprintf("%d + %d = ?", left, right), nil
}

func (c *captchaStore) create(answer string, expiresAt time.Time, now time.Time) (string, error) {
	id, err := captchaID()
	if err != nil {
		return "", err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.challenges == nil {
		c.challenges = make(map[string]captchaChallenge)
	}
	c.pruneLocked(now)
	c.challenges[id] = captchaChallenge{Answer: strings.TrimSpace(answer), ExpiresAt: expiresAt}
	return id, nil
}

func (c *captchaStore) verify(id, answer string, now time.Time) bool {
	id = strings.TrimSpace(id)
	answer = strings.TrimSpace(answer)
	if id == "" || answer == "" {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pruneLocked(now)
	challenge, ok := c.challenges[id]
	if !ok {
		return false
	}
	delete(c.challenges, id)
	return challenge.Answer == answer
}

func (c *captchaStore) pruneLocked(now time.Time) {
	for id, challenge := range c.challenges {
		if !challenge.ExpiresAt.After(now) {
			delete(c.challenges, id)
		}
	}
}

func captchaID() (string, error) {
	buf := make([]byte, 18)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func secureInt(max int64) (int, error) {
	if max < 1 {
		return 0, errors.New("max must be positive")
	}
	value, err := rand.Int(rand.Reader, big.NewInt(max))
	if err != nil {
		return 0, err
	}
	return int(value.Int64()), nil
}
