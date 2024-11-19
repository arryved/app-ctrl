package api

import (
	"encoding/base64"
	"fmt"
	"regexp"
)

var MaxSecretIdLength = 255
var MaxDecodedValueLength = 64 * 1024
var SecretRequestIdPattern = regexp.MustCompile(`^[a-zA-Z0-9-_]+$`)
var SecretRequestOwnerPattern = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@arryved\.com$`)

func SecretRequestCreateValidate(r SecretRequest) error {
	// required for this: id, ownerGroup, value; ownerUser should not be set, will be populated from authn claims
	if r.Id == "" || r.OwnerGroup == "" || len(r.Value) == 0 {
		return fmt.Errorf("one or more required fields missing for create")
	}
	if !(SecretRequestOwnerPattern.MatchString(r.OwnerGroup) && SecretRequestOwnerPattern.MatchString(r.OwnerGroup)) {
		return fmt.Errorf("one or both of ownerUser, ownerGroup isn't a valid email address")
	}
	if !(SecretRequestIdPattern.MatchString(r.Id) && len(r.Id) < MaxSecretIdLength) {
		return fmt.Errorf("id must only containe letters, numbers, hyphens and underscores, max len is %d", MaxSecretIdLength)
	}
	// value should be a b64 string with a size limit
	decodedBytes, decodeErr := validateAndDecodeBase64(r.Value)
	if decodeErr != nil {
		return fmt.Errorf("could not decode value as b64 err=%s", decodeErr)
	}
	if decodedBytes > MaxDecodedValueLength {
		return fmt.Errorf("value decodes to %d bytes, max is %d", decodedBytes, MaxDecodedValueLength)
	}
	return nil
}

func SecretRequestUpdateValidate(r SecretRequest) error {
	// only the value field should be present; id is in URL, owner and group are immutable by convention
	if r.Id != "" || r.OwnerGroup != "" || r.OwnerUser != "" || len(r.Value) == 0 {
		return fmt.Errorf("only value should be present for update, or value is empty")
	}
	// value should be a b64 string with a size limit
	decodedBytes, decodeErr := validateAndDecodeBase64(r.Value)
	if decodeErr != nil {
		return fmt.Errorf("could not decode value as b64 err=%s", decodeErr)
	}
	if decodedBytes > MaxDecodedValueLength {
		return fmt.Errorf("value decodes to %d bytes, max is %d", decodedBytes, MaxDecodedValueLength)
	}
	return nil
}

func validateAndDecodeBase64(b64 string) (int, error) {
	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return 0, err
	}
	return len(decoded), nil
}
