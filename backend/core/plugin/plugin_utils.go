/*
Licensed to the Apache Software Foundation (ASF) under one or more
contributor license agreements.  See the NOTICE file distributed with
this work for additional information regarding copyright ownership.
The ASF licenses this file to You under the Apache License, Version 2.0
(the "License"); you may not use this file except in compliance with
the License.  You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package plugin

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/utils"
)

const EncodeKeyEnvStr = "ENCRYPTION_SECRET"

// gcmNonceSize is the standard nonce size for AES-GCM.
const gcmNonceSize = 12

// Encrypt AES-GCM encrypts plaintext using ENCRYPTION_SECRET, then base64-encodes the result.
// The output format is: base64(nonce || ciphertext || tag).
func Encrypt(encryptionSecret, plainText string) (string, errors.Error) {
	// add suffix to the data part
	inputBytes := append([]byte(plainText), 123, 110, 100, 100, 116, 102, 125)
	// perform encryption
	output, err := AesEncrypt(inputBytes, []byte(encryptionSecret))
	if err != nil {
		return plainText, err
	}
	// Return the result after Base64 processing
	return base64.StdEncoding.EncodeToString(output), nil
}

// Decrypt base64-decodes then AES-GCM decrypts ciphertext using ENCRYPTION_SECRET.
// For backward compatibility, it also attempts AES-CBC decryption if the data looks like legacy format.
func Decrypt(encryptionSecret, encryptedText string) (string, errors.Error) {
	// when encryption key is not set
	if encryptionSecret == "" {
		// return error message
		return encryptedText, errors.Default.New("encryptionSecret is required")
	}

	// Decode Base64
	decodingFromBase64, err1 := base64.StdEncoding.DecodeString(encryptedText)
	if err1 != nil {
		return encryptedText, errors.Convert(err1)
	}
	// perform AES decryption
	output, err2 := AesDecrypt(decodingFromBase64, []byte(encryptionSecret))
	if err2 != nil {
		return encryptedText, err2
	}

	// Verify and remove suffix
	oSize := len(output)
	if oSize >= 7 {
		check := output[oSize-7 : oSize]
		backEnd := []byte{123, 110, 100, 100, 116, 102, 125}
		if string(check) == string(backEnd) {
			output = output[0 : oSize-7]
			// return result
			return string(output), nil
		}
	}
	return "", errors.Default.New("invalid encryptionSecret")
}

// PKCS7Padding PKCS7 padding
func PKCS7Padding(ciphertext []byte, blockSize int) []byte {
	padding := blockSize - len(ciphertext)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padtext...)
}

// PKCS7UnPadding PKCS7 unPadding
func PKCS7UnPadding(origData []byte) []byte {
	length := len(origData)
	if length == 0 {
		return nil
	}
	unpadding := int(origData[length-1])
	if unpadding >= length {
		return nil
	}
	return origData[:(length - unpadding)]
}

// AesEncrypt AES-256-GCM encrypts origData using key.
// The returned bytes are: nonce (12 bytes) || ciphertext || tag.
func AesEncrypt(origData, key []byte) ([]byte, errors.Error) {
	sha256Key := sha256.Sum256(key)
	block, err := aes.NewCipher(sha256Key[:])
	if err != nil {
		return nil, errors.Convert(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, errors.Convert(err)
	}
	nonce := make([]byte, gcmNonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, errors.Convert(err)
	}
	ciphertext := gcm.Seal(nonce, nonce, origData, nil)
	return ciphertext, nil
}

// AesDecrypt decrypts crypted data using key.
// It first tries AES-256-GCM (expects a 12-byte nonce prefix).
// If that fails and the data length is a multiple of the AES block size (legacy CBC format),
// it falls back to AES-256-CBC for backward compatibility.
func AesDecrypt(crypted, key []byte) ([]byte, errors.Error) {
	sha256Key := sha256.Sum256(key)
	block, err := aes.NewCipher(sha256Key[:])
	if err != nil {
		return nil, errors.Convert(err)
	}
	blockSize := block.BlockSize()

	// Try GCM first if the data is long enough to contain a nonce.
	if len(crypted) >= gcmNonceSize+blockSize {
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return nil, errors.Convert(err)
		}
		nonce := crypted[:gcmNonceSize]
		ciphertext := crypted[gcmNonceSize:]
		plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
		if err == nil {
			return plaintext, nil
		}
		// GCM decryption failed; fall through to try legacy CBC.
	}

	// Legacy CBC fallback.
	if len(crypted)%blockSize != 0 {
		return nil, errors.Default.New(fmt.Sprintf("The length of the data to be decrypted is [%d], so cannot match the required block size [%d]", len(crypted), blockSize))
	}

	blockMode := cipher.NewCBCDecrypter(block, sha256Key[:blockSize])
	origData := make([]byte, len(crypted))
	blockMode.CryptBlocks(origData, crypted)
	origData = PKCS7UnPadding(origData)
	return origData, nil
}

// RandomEncryptionSecret will return a random string of length 128
func RandomEncryptionSecret() (string, errors.Error) {
	return utils.RandLetterBytes(128)
}
