//go:build !integration

package secrets

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"testing"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"github.com/stretchr/testify/assert"
	smpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
)

func TestSecretNewClient(t *testing.T) {
	assert := assert.New(t)
	// a noop, just to get started and preserve the import for further test development
	_, err := secretmanager.NewClient(context.Background())
	assert.NoError(err)

	// check if can instantiate the mock client
	client := MockSecretClient{Name: "projects/000000000000/secrets/my-secret-id"}
	assert.NotNil(client)
}

func TestSecretCreateOK(t *testing.T) {
	assert := assert.New(t)
	projectNumber := "000000000000"
	secretId := "my-secret-id"
	valueBytes := []byte("my-secret-value")
	ctx := context.Background()
	// keep this commented out normally; to develop other cases, uncomment this real client, then mock the interaction when done
	// client, err := secretmanager.NewClient(ctx)
	client := MockSecretClient{Name: secretId}

	err := SecretCreate(ctx, client, projectNumber, secretId, valueBytes)

	assert.NoError(err)
}

func TestSecretCreateExists(t *testing.T) {
	assert := assert.New(t)
	projectNumber := "000000000000"
	secretId := "already-exists"
	valueBytes := []byte("my-secret-value")
	ctx := context.Background()
	// keep this commented out normally; to develop other cases, uncomment this real client, then mock the interaction when done
	// client, err := secretmanager.NewClient(ctx)
	client := MockSecretClient{Name: secretId}

	err := SecretCreate(ctx, client, projectNumber, secretId, valueBytes)

	assert.Error(err)
}

func TestSecretIamSet(t *testing.T) {
	assert := assert.New(t)
	secretId := "my-secret-id"
	projectNumber := "000000000000"
	secretName := fmt.Sprintf("projects/%s/secrets/%s", projectNumber, secretId)
	ownerUser := "wwest@arryved.com"
	ownerGroup := "todo@example.com"
	serviceAccounts := []string{
		fmt.Sprintf("%s-compute@developer.gserviceaccount.com", projectNumber),
		"gke-workload-abcd@my-project.iam.gserviceaccount.com",
	}
	ctx := context.Background()
	// keep this commented out normally; to develop other cases, uncomment this real client, then mock the interaction when done
	// client, err := secretmanager.NewClient(ctx)
	client := MockSecretClient{Name: secretId}

	err := SecretIamSet(ctx, client, secretName, ownerUser, ownerGroup, serviceAccounts)

	assert.NoError(err)
}

func TestSecretRead(t *testing.T) {
	assert := assert.New(t)
	secretId := "my-secret-id"
	projectNumber := "000000000000"
	ctx := context.Background()
	// keep this commented out normally; to develop other cases, uncomment this real client, then mock the interaction when done
	// client, err := secretmanager.NewClient(ctx)
	client := MockSecretClient{Name: secretId, Value: []byte("my-secret-value")}
	valueBytes, err := SecretRead(ctx, client, projectNumber, secretId)

	assert.NoError(err)
	assert.Equal([]byte("my-secret-value"), valueBytes)
}

func TestSecretIamGet(t *testing.T) {
	assert := assert.New(t)
	secretId := "my-secret-id"
	projectNumber := "000000000000"
	secretName := fmt.Sprintf("projects/%s/secrets/%s", projectNumber, secretId)
	ownerUser := "wwest@arryved.com"
	ownerGroup := "todo@example.com"
	ctx := context.Background()
	// keep this commented out normally; to develop other cases, uncomment this real client, then mock the interaction when done
	// client, err := secretmanager.NewClient(ctx)
	client := MockSecretClient{Name: secretName, OwnerUser: ownerUser, OwnerGroup: ownerGroup}
	expected := map[string]string{
		"ownerUser":  ownerUser,
		"ownerGroup": ownerGroup,
	}
	principals, err := SecretIamGet(ctx, client, secretName)
	assert.NoError(err)

	assert.Equal(expected, principals)
}

func TestSecretUpdateExists(t *testing.T) {
	assert := assert.New(t)
	secretId := "already-exists"
	projectNumber := "000000000000"
	valueBytes := []byte("my-new-secret-value")
	ctx := context.Background()
	// keep this commented out normally; to develop other cases, uncomment this real client, then mock the interaction when done
	// client, err := secretmanager.NewClient(ctx)
	client := MockSecretClient{Name: secretId}

	err := SecretUpdate(ctx, client, projectNumber, secretId, valueBytes)

	assert.NoError(err)
}

func TestSecretUpdateNoExists(t *testing.T) {
	assert := assert.New(t)
	byteSlice := make([]byte, 20)
	rand.Read(byteSlice)
	secretId := hex.EncodeToString(byteSlice)
	projectNumber := "000000000000"
	valueBytes := []byte("my-new-secret-value")
	ctx := context.Background()
	// keep this commented out normally; to develop other cases, uncomment this real client, then mock the interaction when done
	// client, err := secretmanager.NewClient(ctx)
	client := MockSecretClient{Name: secretId}

	err := SecretUpdate(ctx, client, projectNumber, secretId, valueBytes)

	assert.Error(err)
}

func TestSecretDeleteExists(t *testing.T) {
	assert := assert.New(t)
	secretId := "my-secret-id"
	projectNumber := "000000000000"
	ctx := context.Background()
	// keep this commented out normally; to develop other cases, uncomment this real client, then mock the interaction when done
	// client, err := secretmanager.NewClient(ctx)
	client := MockSecretClient{Name: secretId}

	err := SecretDelete(ctx, client, projectNumber, secretId)

	assert.NoError(err)
}

func TestSecretDeleteNoExists(t *testing.T) {
	assert := assert.New(t)
	byteSlice := make([]byte, 20)
	rand.Read(byteSlice)
	secretId := hex.EncodeToString(byteSlice)
	projectNumber := "000000000000"
	ctx := context.Background()
	// keep this commented out normally; to develop other cases, uncomment this real client, then mock the interaction when done
	// client, err := secretmanager.NewClient(ctx)
	client := MockSecretClient{Name: secretId}

	err := SecretDelete(ctx, client, projectNumber, secretId)

	assert.Error(err)
}

func TestSecretList(t *testing.T) {
	assert := assert.New(t)
	projectNumber := "676571955389"
	ctx := context.Background()
	// keep this commented out normally; to develop other cases, uncomment this real client, then mock the interaction when done
	// client, err := secretmanager.NewClient(ctx)
	client := MockSecretClient{
		OwnerUser:  "some-user@arryved.com",
		OwnerGroup: "some-group@arryved.com",
		SecretsList: []*smpb.Secret{
			// API will send them pre-sorted by CreateTime, with most recent timestamp first
			&smpb.Secret{
				Name: "projects/project-id/secrets/secret1",
				CreateTime: &timestamppb.Timestamp{
					Seconds: int64(1724043037),
					Nanos:   int32(9875000),
				},
			},
			&smpb.Secret{
				Name: "projects/project-id/secrets/secret2",
				CreateTime: &timestamppb.Timestamp{
					Seconds: int64(1724043036),
					Nanos:   int32(1235000),
				},
			},
		},
	}

	list, err := SecretList(ctx, client, projectNumber)

	// expect no error and two SecretEntry values; this type abstracts away the backing store/API
	// so it can be changed out for something else and hide implementation details/set opinions
	assert.NoError(err)
	assert.Len(list, 2)
	assert.Equal(SecretEntry{
		Urn:            "urn:arryved:secret:secret1",
		CreatedEpochNs: int64(1724043037009875000),
		OwnerGroup:     "some-group@arryved.com",
		OwnerUser:      "some-user@arryved.com",
	}, list[0])
	assert.Equal(SecretEntry{
		Urn:            "urn:arryved:secret:secret2",
		CreatedEpochNs: int64(1724043036001235000),
		OwnerGroup:     "some-group@arryved.com",
		OwnerUser:      "some-user@arryved.com",
	}, list[1])
}
