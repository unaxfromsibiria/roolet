package cryptosupport

import (
	"crypto/rsa"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"roolet/helpers"
	"roolet/options"
	"strings"

	"github.com/dgrijalva/jwt-go"
)

// Howto create key? Use openssl :)
// $openssl genpkey -outform PEM -algorithm RSA -out key.priv -pkeyopt rsa_keygen_bits:1024
// and public key extract from it
// $openssl rsa -in key.priv -out key.pub -pubout

// simple ckeck token
func Check(key *rsa.PublicKey, token string) error {
	parts := strings.Split(token, ".")
	if len(parts) == 3 {
		if _, err := base64.URLEncoding.DecodeString(parts[2]); err == nil {
			if err := jwt.SigningMethodRS256.Verify(strings.Join(parts[0:2], "."), parts[2], key); err != nil {
				return err
			}
		} else {
			return errors.New(fmt.Sprintf("Base64 decode problem: %s with: '%s'.", err, parts[2]))
		}
	} else {
		return errors.New("Write a full token as tools data (3 parts).")
	}
	return nil
}

// Test create token from command line.
func JwtCreate(data string, option *options.SysOption) {
	if key, err := option.GetPrivKey(); err == nil {
		parts := strings.Split(data, ".")
		if len(parts) == 2 {
			data := []string{
				base64.StdEncoding.EncodeToString([]byte(parts[0])),
				base64.StdEncoding.EncodeToString([]byte(parts[1]))}

			sig, err := jwt.SigningMethodRS256.Sign(strings.Join(data, "."), key)
			if err == nil {
				log.Printf(
					"\nSignature: %s\n\nToken: %s\n",
					sig,
					strings.Join(append(data, sig), "."))
			} else {
				log.Fatal(err)
			}
		} else {
			log.Println("Write two parts as tools data, format: 'header.payload'")
		}
	} else {
		log.Fatalf("Open key problem: %s\n", err)
	}
}

// Test check token from command line.
func JwtCheck(data string, option *options.SysOption) {
	if key, err := option.GetPubKey(); err == nil {
		parts := strings.Split(data, ".")
		if len(parts) == 3 {
			if sigDta, err := base64.StdEncoding.DecodeString(parts[2]); err == nil {
				sig := string(sigDta)
				err := jwt.SigningMethodRS256.Verify(strings.Join(parts[0:2], "."), sig, key)
				if err == nil {
					log.Printf("\nCheck passed!\nSignature: %s\n", sig)
				} else {
					log.Fatal(err)
				}
			} else {
				log.Fatalf("Base64 decode problem: %s with: '%s'\n", err, parts[2])
			}
		} else {
			log.Println("Write a full token as tools data (3 parts)")
		}
	} else {
		log.Fatalf("Open key problem: %s\n", err)
	}
}

// Simple check keys.
func KeysSimpleCheck(data string, option *options.SysOption) {
	if privKey, err := option.GetPrivKey(); err == nil {
		if pubKey, err := option.GetPubKey(); err == nil {
			rand := helpers.NewSystemRandom()
			mainPart := fmt.Sprint(
				"%s.%s",
				base64.StdEncoding.EncodeToString([]byte(rand.CreatePassword(64))),
				base64.StdEncoding.EncodeToString([]byte(rand.CreatePassword(96))))

			sig, err := jwt.SigningMethodRS256.Sign(mainPart, privKey)
			if err == nil {
				err := jwt.SigningMethodRS256.Verify(mainPart, sig, pubKey)
				if err == nil {
					log.Printf("Keys from '%s' is correct\n", option.KeyDir)
				}
			} else {
				log.Fatalf("Can't ctrate signature: %s\n", err)
			}
		} else {
			log.Fatalf("Can't open public key! Error: %s\n", err)
		}
	} else {
		log.Fatalf("Can't open private key! Error: %s\n", err)
	}
}
