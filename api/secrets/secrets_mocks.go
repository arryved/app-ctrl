package secrets

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"time"
	"unsafe"

	"cloud.google.com/go/iam"
	"cloud.google.com/go/iam/apiv1/iampb"
	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"github.com/googleapis/gax-go/v2"
	"google.golang.org/api/iterator"
	smpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
)

type MockSecretClient struct {
	Name        string
	Value       []byte
	OwnerUser   string
	OwnerGroup  string
	SecretsList []*smpb.Secret
}

func (m MockSecretClient) CreateSecret(
	ctx context.Context, req *smpb.CreateSecretRequest, options ...gax.CallOption) (*smpb.Secret, error) {
	name := fmt.Sprintf("projects/000000000000/secrets/%s", m.Name)

	if m.Name == "already-exists" {
		return nil, fmt.Errorf("[mock response] secret name=%s already exists", name)
	}

	mockResult := &smpb.Secret{
		CreateTime: timestamppb.New(time.Now()),
		Replication: &smpb.Replication{
			Replication: &smpb.Replication_Automatic_{
				Automatic: &smpb.Replication_Automatic{},
			},
		},
		Name: name,
		Etag: "\"861fa19e22af28\"",
	}
	return mockResult, nil
}

func (m MockSecretClient) DeleteSecret(
	ctx context.Context, req *smpb.DeleteSecretRequest, options ...gax.CallOption) error {
	hex20Matcher := regexp.MustCompile(`^[a-fA-F0-9]{40}$`)
	if hex20Matcher.MatchString(m.Name) {
		return fmt.Errorf("the secret doesn't exist")
	}
	return nil
}

func (m MockSecretClient) AccessSecretVersion(
	ctx context.Context, req *smpb.AccessSecretVersionRequest, options ...gax.CallOption) (*smpb.AccessSecretVersionResponse, error) {
	mockResult := &smpb.AccessSecretVersionResponse{
		Name: fmt.Sprintf("%s/versions/1", m.Name),
		Payload: &smpb.SecretPayload{
			Data: m.Value,
		},
	}
	return mockResult, nil
}

func (m MockSecretClient) AddSecretVersion(
	ctx context.Context, req *smpb.AddSecretVersionRequest, options ...gax.CallOption) (*smpb.SecretVersion, error) {
	hex20Matcher := regexp.MustCompile(`^[a-fA-F0-9]{40}$`)
	if hex20Matcher.MatchString(m.Name) {
		return nil, fmt.Errorf("the secret doesn't exist")
	}
	mockResult := &smpb.SecretVersion{}
	return mockResult, nil
}

func (m MockSecretClient) IAM(name string) *iam.Handle {
	return iam.InternalNewHandleClient(m, name)
}

func (m MockSecretClient) Get(ctx context.Context, name string) (*iampb.Policy, error) {
	return nil, nil
}

func (m MockSecretClient) GetIamPolicy(ctx context.Context, req *iampb.GetIamPolicyRequest, options ...gax.CallOption) (*iampb.Policy, error) {
	mockResult := &iampb.Policy{
		Bindings: []*iampb.Binding{
			&iampb.Binding{
				Members: []string{
					fmt.Sprintf("user:%s", m.OwnerUser),
					fmt.Sprintf("group:%s", m.OwnerGroup),
					fmt.Sprintf("serviceAccount:000000000000-compute@developer.gserviceaccount.com"),
				},
				Role: accessorRole,
			},
		},
	}
	return mockResult, nil
}

func (m MockSecretClient) GetWithVersion(ctx context.Context, name string, version int32) (*iampb.Policy, error) {
	return nil, nil
}

func (m MockSecretClient) ListSecrets(ctx context.Context, req *smpb.ListSecretsRequest, options ...gax.CallOption) *secretmanager.SecretIterator {
	secrets := m.SecretsList
	it := &secretmanager.SecretIterator{
		InternalFetch: func(pageSize int, pageToken string) ([]*smpb.Secret, string, error) {
			return secrets, "", nil
		},
		Response: &smpb.ListSecretsResponse{
			Secrets:   secrets,
			TotalSize: int32(len(secrets)),
		},
	}
	// reflection monkeypatch for SecretIterator.items field, no obvious way to inject one
	val := reflect.ValueOf(it).Elem()
	itemsField := val.FieldByName("items")
	if !itemsField.CanSet() {
		itemsField = reflect.NewAt(itemsField.Type(), unsafe.Pointer(itemsField.UnsafeAddr())).Elem()
	}
	itemsField.Set(reflect.ValueOf(secrets))

	// reflection monkeypatch for SecretIterator.nextFunc field, no obvious way to inject one
	index := 0
	nextPatch := func() error {
		if index < len(secrets) {
			it.Response = secrets
			index++
			return nil
		}
		return iterator.Done
	}
	field := val.FieldByName("nextFunc")
	if !field.CanSet() {
		field = reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
	}
	field.Set(reflect.ValueOf(nextPatch))

	return it
}

func (m MockSecretClient) Set(ctx context.Context, name string, policy *iampb.Policy) error {
	return nil
}

func (m MockSecretClient) Test(ctx context.Context, value string, values []string) ([]string, error) {
	return []string{}, nil
}
