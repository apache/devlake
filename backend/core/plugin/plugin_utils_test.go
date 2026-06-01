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
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"testing"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/stretchr/testify/assert"
)

func TestEncodeAndDecode(t *testing.T) {
	TestStr := "The string for testing"
	var err error

	var TestEncode string
	var TestDecode string

	encryptionSecret, _ := RandomEncryptionSecret()
	// encryption test
	TestEncode, err = Encrypt(encryptionSecret, TestStr)
	assert.Empty(t, err)

	// decrypt test
	TestDecode, err = Decrypt(encryptionSecret, TestEncode)
	assert.Empty(t, err)

	// Verify decryption result
	assert.Equal(t, string(TestDecode), TestStr)
}

func TestEncode(t *testing.T) {
	encryptionSecret, _ := RandomEncryptionSecret()
	type args struct {
		Input string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			"",
			args{"bGlhbmcuemhhbmdAbWVyaWNvLmRldjprYUU2eWpNY1VYV2FCNUhIS3BGRkQ1RTg="},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Encrypt(encryptionSecret, tt.args.Input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Encode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			t.Log(got)
		})
	}
}

func TestGCMEncDec(t *testing.T) {
	TestStr := "The string for testing"
	encryptionSecret, _ := RandomEncryptionSecret()

	// Encrypt with the new GCM format.
	newCiphertext, err := Encrypt(encryptionSecret, TestStr)
	assert.Empty(t, err)

	// Decrypt the new format.
	decodedNew, err := Decrypt(encryptionSecret, newCiphertext)
	assert.Empty(t, err)
	assert.Equal(t, TestStr, decodedNew)

	// Ensure two encryptions of the same plaintext produce different ciphertexts (random nonce).
	newCiphertext2, err := Encrypt(encryptionSecret, TestStr)
	assert.Empty(t, err)
	assert.NotEqual(t, newCiphertext, newCiphertext2)
}

// AesEncrypt AES encryption, CBC
func oldAesEncrypt(origData, key []byte) ([]byte, errors.Error) {
	// data alignment fill and encryption
	sha256Key := sha256.Sum256(key)
	key = sha256Key[:]
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.Convert(err)
	}
	// data alignment fill and encryption
	blockSize := block.BlockSize()
	origData = PKCS7Padding(origData, blockSize)
	blockMode := cipher.NewCBCEncrypter(block, key[:blockSize])
	crypted := make([]byte, len(origData))
	blockMode.CryptBlocks(crypted, origData)
	return crypted, nil
}

func oldEncrypt(encryptionSecret, plainText string) (string, errors.Error) {
	// add suffix to the data part
	inputBytes := append([]byte(plainText), 123, 110, 100, 100, 116, 102, 125)
	// perform encryption
	output, err := oldAesEncrypt(inputBytes, []byte(encryptionSecret))
	if err != nil {
		return plainText, err
	}
	// Return the result after Base64 processing
	return base64.StdEncoding.EncodeToString(output), nil
}

func TestBackwardCompatibility(t *testing.T) {
	TestStr := "The string for testing"
	encryptionSecret, _ := RandomEncryptionSecret()

	// Encrypt with the new GCM format.
	newCiphertext, err := oldEncrypt(encryptionSecret, TestStr)
	assert.Empty(t, err)

	// Decrypt the new format.
	decodedNew, err := Decrypt(encryptionSecret, newCiphertext)
	assert.Empty(t, err)
	assert.Equal(t, TestStr, decodedNew)

}
