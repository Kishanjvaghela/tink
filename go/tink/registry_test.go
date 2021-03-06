// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
////////////////////////////////////////////////////////////////////////////////

package tink_test

import (
	"fmt"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/google/tink/go/aead"
	"github.com/google/tink/go/mac"
	subtleAEAD "github.com/google/tink/go/subtle/aead"
	subtleMac "github.com/google/tink/go/subtle/mac"
	"github.com/google/tink/go/testkeysethandle"
	"github.com/google/tink/go/testutil"
	"github.com/google/tink/go/tink"
	gcmpb "github.com/google/tink/proto/aes_gcm_go_proto"
	commonpb "github.com/google/tink/proto/common_go_proto"
	hmacpb "github.com/google/tink/proto/hmac_go_proto"
	tinkpb "github.com/google/tink/proto/tink_go_proto"
)

func setupRegistryTests() {
	if err := mac.Register(); err != nil {
		panic(fmt.Sprintf("cannot register MAC key types: %v", err))
	}

	if err := aead.Register(); err != nil {
		panic(fmt.Sprintf("cannot register AEAD key types: %v", err))
	}
}

func TestRegistryBasic(t *testing.T) {
	// try to put a HMACKeyManager
	hmacManager := mac.NewHMACKeyManager()
	typeURL := mac.HMACTypeURL
	tink.RegisterKeyManager(hmacManager)
	tmp, existed := tink.GetKeyManager(typeURL)
	if existed != nil {
		t.Errorf("a HMACKeyManager should be found")
	}
	var _ = tmp.(*mac.HMACKeyManager)
	// Get type url that doesn't exist
	if _, existed := tink.GetKeyManager("some url"); existed == nil {
		t.Errorf("unknown typeURL shouldn't exist in the map")
	}
}

func TestRegisterKeyManager(t *testing.T) {
	var km tink.KeyManager
	var err error
	// register mac and aead types.
	setupRegistryTests()
	// get HMACKeyManager
	km, err = tink.GetKeyManager(mac.HMACTypeURL)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	var _ *mac.HMACKeyManager = km.(*mac.HMACKeyManager)
	// get AESGCMKeyManager
	km, err = tink.GetKeyManager(aead.AESGCMTypeURL)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	var _ *aead.AESGCMKeyManager = km.(*aead.AESGCMKeyManager)
	// some random typeurl
	if _, err = tink.GetKeyManager("some url"); err == nil {
		t.Errorf("expect an error when a type url doesn't exist in the registry")
	}
}

func TestRegisterKeyManagerWithCollision(t *testing.T) {
	// register mac and aead types.
	setupRegistryTests()
	// dummyKeyManager's typeURL is equal to that of AESGCM
	var dummyKeyManager tink.KeyManager = new(testutil.DummyAEADKeyManager)
	// this should not overwrite the existing manager.
	err := tink.RegisterKeyManager(dummyKeyManager)
	if err != nil {
		t.Errorf("AES_GCM_TYPE_URL shouldn't be registered again")
	}
	km, err := tink.GetKeyManager(aead.AESGCMTypeURL)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	var _ *aead.AESGCMKeyManager = km.(*aead.AESGCMKeyManager)
}

func TestNewKeyData(t *testing.T) {
	setupRegistryTests()
	// new Keydata from a Hmac KeyTemplate
	keyData, err := tink.NewKeyData(mac.HMACSHA256Tag128KeyTemplate())
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	if keyData.TypeUrl != mac.HMACTypeURL {
		t.Errorf("invalid key data")
	}
	key := new(hmacpb.HmacKey)
	if err := proto.Unmarshal(keyData.Value, key); err != nil {
		t.Errorf("unexpected error when unmarshal HmacKey: %s", err)
	}
	// nil
	if _, err := tink.NewKeyData(nil); err == nil {
		t.Errorf("expect an error when key template is nil")
	}
	// unregistered type url
	template := &tinkpb.KeyTemplate{TypeUrl: "some url", Value: []byte{0}}
	if _, err := tink.NewKeyData(template); err == nil {
		t.Errorf("expect an error when key template contains unregistered typeURL")
	}
}

func TestNewKey(t *testing.T) {
	setupRegistryTests()
	// aead template
	aesGcmTemplate := aead.AES128GCMKeyTemplate()
	key, err := tink.NewKey(aesGcmTemplate)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	var aesGcmKey *gcmpb.AesGcmKey = key.(*gcmpb.AesGcmKey)
	aesGcmFormat := new(gcmpb.AesGcmKeyFormat)
	if err := proto.Unmarshal(aesGcmTemplate.Value, aesGcmFormat); err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	if aesGcmFormat.KeySize != uint32(len(aesGcmKey.KeyValue)) {
		t.Errorf("key doesn't match template")
	}
	//nil
	if _, err := tink.NewKey(nil); err == nil {
		t.Errorf("expect an error when key template is nil")
	}
	// unregistered type url
	template := &tinkpb.KeyTemplate{TypeUrl: "some url", Value: []byte{0}}
	if _, err := tink.NewKey(template); err == nil {
		t.Errorf("expect an error when key template is not registered")
	}
}

func TestPrimitiveFromKeyData(t *testing.T) {
	setupRegistryTests()
	// hmac keydata
	keyData := testutil.NewHMACKeyData(commonpb.HashType_SHA256, 16)
	p, err := tink.PrimitiveFromKeyData(keyData)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	var _ *subtleMac.HMAC = p.(*subtleMac.HMAC)
	// unregistered url
	keyData.TypeUrl = "some url"
	if _, err := tink.PrimitiveFromKeyData(keyData); err == nil {
		t.Errorf("expect an error when typeURL has not been registered")
	}
	// unmatched url
	keyData.TypeUrl = aead.AESGCMTypeURL
	if _, err := tink.PrimitiveFromKeyData(keyData); err == nil {
		t.Errorf("expect an error when typeURL doesn't match key")
	}
	// nil
	if _, err := tink.PrimitiveFromKeyData(nil); err == nil {
		t.Errorf("expect an error when key data is nil")
	}
}

func TestPrimitive(t *testing.T) {
	setupRegistryTests()
	// hmac key
	key := testutil.NewHMACKey(commonpb.HashType_SHA256, 16)
	serializedKey, _ := proto.Marshal(key)
	p, err := tink.Primitive(mac.HMACTypeURL, serializedKey)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	var _ *subtleMac.HMAC = p.(*subtleMac.HMAC)
	// unregistered url
	if _, err := tink.Primitive("some url", serializedKey); err == nil {
		t.Errorf("expect an error when typeURL has not been registered")
	}
	// unmatched url
	if _, err := tink.Primitive(aead.AESGCMTypeURL, serializedKey); err == nil {
		t.Errorf("expect an error when typeURL doesn't match key")
	}
	// void key
	if _, err := tink.Primitive(aead.AESGCMTypeURL, nil); err == nil {
		t.Errorf("expect an error when key is nil")
	}
	if _, err := tink.Primitive(aead.AESGCMTypeURL, []byte{}); err == nil {
		t.Errorf("expect an error when key is nil")
	}
	if _, err := tink.Primitive(aead.AESGCMTypeURL, []byte{0}); err == nil {
		t.Errorf("expect an error when key is nil")
	}
}

func TestPrimitives(t *testing.T) {
	setupRegistryTests()
	// valid input
	template1 := aead.AES128GCMKeyTemplate()
	template2 := aead.AES256GCMKeyTemplate()
	keyData1, _ := tink.NewKeyData(template1)
	keyData2, _ := tink.NewKeyData(template2)
	keyset := tink.CreateKeyset(2, []*tinkpb.Keyset_Key{
		tink.CreateKey(keyData1, tinkpb.KeyStatusType_ENABLED, 1, tinkpb.OutputPrefixType_TINK),
		tink.CreateKey(keyData2, tinkpb.KeyStatusType_ENABLED, 2, tinkpb.OutputPrefixType_TINK),
	})
	handle, _ := testkeysethandle.KeysetHandle(keyset)
	ps, err := tink.Primitives(handle)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	var aesGcm *subtleAEAD.AESGCM = ps.Primary.Primitive.(*subtleAEAD.AESGCM)
	if len(aesGcm.Key) != 32 {
		t.Errorf("primitive doesn't match input keyset handle")
	}
	// custom manager
	customManager := new(testutil.DummyAEADKeyManager)
	ps, err = tink.PrimitivesWithKeyManager(handle, customManager)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	var _ *testutil.DummyAEAD = ps.Primary.Primitive.(*testutil.DummyAEAD)
	// keysethandle is nil
	if _, err := tink.Primitives(nil); err == nil {
		t.Errorf("expect an error when keysethandle is nil")
	}
	// keyset is empty
	keyset = tink.CreateKeyset(1, []*tinkpb.Keyset_Key{})
	handle, _ = testkeysethandle.KeysetHandle(keyset)
	if _, err := tink.Primitives(handle); err == nil {
		t.Errorf("expect an error when keyset is empty")
	}
	keyset = tink.CreateKeyset(1, nil)
	handle, _ = testkeysethandle.KeysetHandle(keyset)
	if _, err := tink.Primitives(handle); err == nil {
		t.Errorf("expect an error when keyset is empty")
	}
	// no primary key
	keyset = tink.CreateKeyset(3, []*tinkpb.Keyset_Key{
		tink.CreateKey(keyData1, tinkpb.KeyStatusType_ENABLED, 1, tinkpb.OutputPrefixType_TINK),
		tink.CreateKey(keyData2, tinkpb.KeyStatusType_ENABLED, 2, tinkpb.OutputPrefixType_TINK),
	})
	handle, _ = testkeysethandle.KeysetHandle(keyset)
	if _, err := tink.Primitives(handle); err == nil {
		t.Errorf("expect an error when there is no primary key")
	}
	// there is primary key but it is disabled
	keyset = tink.CreateKeyset(1, []*tinkpb.Keyset_Key{
		tink.CreateKey(keyData1, tinkpb.KeyStatusType_DISABLED, 1, tinkpb.OutputPrefixType_TINK),
		tink.CreateKey(keyData2, tinkpb.KeyStatusType_ENABLED, 2, tinkpb.OutputPrefixType_TINK),
	})
	handle, _ = testkeysethandle.KeysetHandle(keyset)
	if _, err := tink.Primitives(handle); err == nil {
		t.Errorf("expect an error when primary key is disabled")
	}
	// multiple primary keys
	keyset = tink.CreateKeyset(1, []*tinkpb.Keyset_Key{
		tink.CreateKey(keyData1, tinkpb.KeyStatusType_ENABLED, 1, tinkpb.OutputPrefixType_TINK),
		tink.CreateKey(keyData2, tinkpb.KeyStatusType_ENABLED, 1, tinkpb.OutputPrefixType_TINK),
	})
	handle, _ = testkeysethandle.KeysetHandle(keyset)
	if _, err := tink.Primitives(handle); err == nil {
		t.Errorf("expect an error when there are multiple primary keys")
	}
}
