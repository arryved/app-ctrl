package secrets

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"

	"cloud.google.com/go/iam"
	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"github.com/googleapis/gax-go/v2"
	log "github.com/sirupsen/logrus"
	"google.golang.org/api/iterator"
	smpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
	iampb "google.golang.org/genproto/googleapis/iam/v1"

	"github.com/arryved/app-ctrl/api/config"
)

// This role is used as a hint; users with the role will be restricted to access-only in other tools, but app-control
// will allow them to do more (full CRUD)
const accessorRole = "roles/secretmanager.secretAccessor"

// This limits how many IAM requests can happen at once. This won't scale past a few hundred secrets, but caching isn't
// necessary at the outset
const listIamConcurrency = 32

// Body format for an app-control-api secret request
type SecretRequest struct {
	Id         string `json:"id"`         // a secret name matching `^[a-zA-Z0-9-_]+$`; 255 byte max length
	OwnerGroup string `json:"ownerGroup"` // just the plain email address
	OwnerUser  string `json:"ownerUser"`  // just the plain email address
	Value      string `json:"value"`      // expects b64-encoded bytes in a json string; decoded size limit is 64k bytes
}

// Abstraction for an app-control-api secret. Hides implementation details. Think before allowing them to leak in.
type SecretEntry struct {
	Urn string `json:"urn"`
	// I don't think this code will be here in 290 years so int64 is probably fine
	// as of 2024 the delivered precision from GCP is ms, this just honoring their aspirational precision format (ns, int64 + int32)
	CreatedEpochNs int64  `json:"createdEpochNs"`
	OwnerGroup     string `json:"ownerGroup"`
	OwnerUser      string `json:"ownerUser"`
}

// Generalized interface for the SecretManager methods we use in the API; allows client mocking
type SecretManagerClient interface {
	AccessSecretVersion(context.Context, *smpb.AccessSecretVersionRequest, ...gax.CallOption) (*smpb.AccessSecretVersionResponse, error)
	AddSecretVersion(context.Context, *smpb.AddSecretVersionRequest, ...gax.CallOption) (*smpb.SecretVersion, error)
	CreateSecret(context.Context, *smpb.CreateSecretRequest, ...gax.CallOption) (*smpb.Secret, error)
	DeleteSecret(context.Context, *smpb.DeleteSecretRequest, ...gax.CallOption) error
	GetIamPolicy(context.Context, *iampb.GetIamPolicyRequest, ...gax.CallOption) (*iampb.Policy, error)
	IAM(string) *iam.Handle
	ListSecrets(context.Context, *smpb.ListSecretsRequest, ...gax.CallOption) *secretmanager.SecretIterator
}

// Generalized interface for the IAM methods we use in the API; allows client mocking
type IamHandle interface {
	Policy(context.Context) (*iam.Policy, error)
	SetPolicy(context.Context, *iam.Policy) error
	TestPermissions(context.Context, []string) ([]string, error)
	V3() *iam.Handle3
}

// CREATE unit. Does not do auth by itself; use the RBAC module in concert
func SecretCreate(ctx context.Context, client SecretManagerClient, projectNumber, secretId string, valueBytes []byte) error {
	// add a secret
	req := &smpb.CreateSecretRequest{
		Parent:   fmt.Sprintf("projects/%s", projectNumber),
		SecretId: secretId,
		Secret: &smpb.Secret{
			Replication: &smpb.Replication{
				Replication: &smpb.Replication_Automatic_{
					Automatic: &smpb.Replication_Automatic{},
				},
			},
		},
	}
	result, err := client.CreateSecret(ctx, req)
	if err != nil {
		return err
	}
	log.Infof("Created secret project=%s secretId=%s result=%v", projectNumber, secretId, result)

	// add a version
	err = addSecretVersion(ctx, client, result.Name, valueBytes)
	if err != nil {
		return err
	}

	return nil
}

func addSecretVersion(ctx context.Context, client SecretManagerClient, secretName string, valueBytes []byte) error {
	addSecretVersionReq := &smpb.AddSecretVersionRequest{
		Parent: secretName,
		Payload: &smpb.SecretPayload{
			Data: valueBytes,
		},
	}
	version, err := client.AddSecretVersion(ctx, addSecretVersionReq)
	if err != nil {
		return err
	}
	log.Infof("Added secret version secretName=%s versionName=%s", secretName, version.Name)
	return nil
}

// Set Secret IAM unit. Does not authorize; this grants permissions. Use the RBAC module in concert with this
//
// bind IAM permissions to the secret:
// - secret accessor for default compute account
// - secret accessor for workload identity account(s)
// - secret accessor for the creator principal
// - secret accessor for a supplied owner group principal
// ...Create/Update/Delete will be allowed through the tool for any of these principals
//
//	this will let people see secrets in GCP console but they'll have to use app-control to C/U/D
func SecretIamSet(
	ctx context.Context, client SecretManagerClient, secretName string, ownerUser, ownerGroup string, serviceAccounts []string) error {
	handle := client.IAM(secretName)
	log.Debugf("handle=%v", handle)
	policy, err := handle.Policy(ctx)
	if err != nil {
		return err
	}

	role := iam.RoleName(accessorRole)

	for _, serviceAccount := range serviceAccounts {
		// Grant accessor permissions
		member := fmt.Sprintf("serviceAccount:%s", serviceAccount)
		policy.Add(member, role)
	}

	// Grant the owner user permission
	memberUser := fmt.Sprintf("user:%s", ownerUser)
	policy.Add(memberUser, role)

	// Grant the owner group permissions
	groupMember := fmt.Sprintf("group:%s", ownerGroup)
	policy.Add(groupMember, role)

	err = handle.SetPolicy(ctx, policy)
	if err != nil {
		return err
	}
	log.Infof("bound IAM roles to secret for policy=%v role=%s", policy, role)
	return nil
}

// READ unit. Does not authorize; this grants permissions. Use the RBAC module in concert with this
func SecretRead(
	ctx context.Context, client SecretManagerClient, projectNumber, secretId string) ([]byte, error) {
	accessRequest := &smpb.AccessSecretVersionRequest{
		Name: fmt.Sprintf("projects/%s/secrets/%s/versions/latest", projectNumber, secretId),
	}
	result, err := client.AccessSecretVersion(ctx, accessRequest)
	if err != nil {
		return []byte{}, err
	}
	valueBytes := result.Payload.Data

	// GCP uses crc32c in their console UI but you can see the secret there anyway... safer to use salt w/ sha256 when logging
	salt := make([]byte, 16)
	_, err = io.ReadFull(rand.Reader, salt)
	if err != nil {
		return []byte{}, err
	}
	saltedValue := append(salt, valueBytes...)
	hash := sha256.Sum256(saltedValue)
	log.Infof("result type=%T name=%s, salt=%x, sha256=%x", result, result.Name, salt, hash)

	return valueBytes, nil
}

// Secret IAM get unit. Retrieves IAM bindings for a secret. To be used in concert with RBAC module for authorization.
func SecretIamGet(
	ctx context.Context, client SecretManagerClient, secretName string) (map[string]string, error) {
	req := &iampb.GetIamPolicyRequest{
		Resource: secretName,
	}
	policy, err := client.GetIamPolicy(ctx, req)
	if err != nil {
		return map[string]string{}, err
	}

	members := map[string]string{
		"ownerUser":  "",
		"ownerGroup": "",
	}
	for _, binding := range policy.Bindings {
		// app-control keys off of accessor role as a hint... ignores other roles, the ideas is to funnel users into the API
		// and hide the implementation specifics
		if binding.Role == accessorRole {
			for _, member := range binding.Members {
				// exclude service accounts from the list to prevent abuse
				// note: if multiple users, last one wins. by convention (initially) only one user accessor binding per secret
				if strings.HasPrefix(member, "user") {
					members["ownerUser"] = normalizeMember(member)
				}
				// note: if multiple groups, last one wins. by convention (initially) only one group accessor binding per secret
				if strings.HasPrefix(member, "group") {
					members["ownerGroup"] = normalizeMember(member)
				}
			}
		}
	}
	return members, nil
}

// UPDATE unit. Does not authorize. Use the RBAC module in concert with this
func SecretUpdate(ctx context.Context, client SecretManagerClient, projectNumber, secretId string, valueBytes []byte) error {
	// reconstruct the name
	secretName := fmt.Sprintf("projects/%s/secrets/%s", projectNumber, secretId)
	log.Infof("Attempting to update secret=%s", secretName)

	// add a version
	err := addSecretVersion(ctx, client, secretName, valueBytes)
	if err != nil {
		return err
	}
	return nil
}

// DELETE unit. Does not authorize. Use the RBAC module in concert with this
func SecretDelete(ctx context.Context, client SecretManagerClient, projectNumber, secretId string) error {
	secretName := fmt.Sprintf("projects/%s/secrets/%s", projectNumber, secretId)
	log.Infof("Attempting to delete secret=%s", secretName)

	req := &smpb.DeleteSecretRequest{
		Name: secretName,
	}

	err := client.DeleteSecret(ctx, req)
	if err != nil {
		return err
	}
	log.Infof("Delete successful for secret=%s", secretName)
	return nil
}

// LIST unit. Does not authorize or authenticate. Use authentication to guard this at a minimum.
func SecretList(ctx context.Context, client SecretManagerClient, projectNumber string) ([]SecretEntry, error) {
	req := &smpb.ListSecretsRequest{
		Parent: fmt.Sprintf("projects/%s", projectNumber),
	}
	listIter := client.ListSecrets(ctx, req)

	jobs := make(chan *smpb.Secret, listIamConcurrency)
	results := make(chan SecretEntry, listIamConcurrency)
	var wg sync.WaitGroup

	for i := 0; i < listIamConcurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for secret := range jobs {
				ownerUser := ""
				ownerGroup := ""
				policy, err := client.GetIamPolicy(ctx, &iampb.GetIamPolicyRequest{Resource: secret.Name})
				if err != nil {
					log.Warnf("could not get policy for secret=%s, ownership not determined", secret.Name)
				} else {
					// only supporting one each by convention, last found of each wins
					for _, binding := range policy.Bindings {
						if binding.Role == accessorRole {
							for _, member := range binding.Members {
								if strings.HasPrefix(member, "user") {
									ownerUser = normalizeMember(member)
								}
								if strings.HasPrefix(member, "group") {
									ownerGroup = normalizeMember(member)
								}
							}
						}
					}
				}
				nameParts := strings.Split(secret.Name, "/")
				urn := fmt.Sprintf("urn:arryved:secret:%s", nameParts[len(nameParts)-1])
				result := SecretEntry{
					Urn:            urn,
					OwnerGroup:     ownerGroup,
					OwnerUser:      ownerUser,
					CreatedEpochNs: secret.CreateTime.Seconds*1e9 + int64(secret.CreateTime.Nanos),
				}
				if ownerUser == "" {
					log.Warnf("secret %s has no user owner", result.Urn)
				}
				if ownerGroup == "" {
					log.Warnf("secret %s has no group owner", result.Urn)
				}
				results <- result
			}
		}()
	}

	for {
		secret, err := listIter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return []SecretEntry{}, err
		}
		jobs <- secret
	}
	close(jobs)
	wg.Wait()
	close(results)

	resultList := []SecretEntry{}
	for result := range results {
		resultList = append(resultList, result)
	}
	sort.Slice(resultList, func(i, j int) bool {
		return resultList[i].CreatedEpochNs > resultList[j].CreatedEpochNs
	})
	return resultList, nil
}

func SecretsAuthorizer(
	ctx context.Context, cfg *config.Config, client interface{},
	principal config.PrincipalUrn, action config.Permission, target string) error {
	// get the iam details
	projectNumber := ctx.Value("projectNumber").(string)
	mutation := action == config.SecretsUpdate || action == config.SecretsDelete

	if mutation {
		// UPDATE | DELETE - allowed only for ownerUser or a member of ownerGroup
		secretName := fmt.Sprintf("projects/%s/secrets/%s", projectNumber, strings.Split(target, ":")[3])
		principalMap, err := SecretIamGet(ctx, client.(SecretManagerClient), secretName)
		if err != nil {
			return err
		}
		log.Infof("authorizing principal=%s action=%s target=%s", principal, action, target)
		log.Infof("iam for secret=%s found ownerGroup=%s ownerUser=%s", secretName, principalMap["ownerGroup"], principalMap["ownerUser"])
	} else {
		// CREATE | LIST - anyone can do these
		return nil
	}
	return nil
}

func normalizeMember(str string) string {
	fields := strings.Split(str, ":")
	if len(fields) > 1 {
		return fields[1]
	}
	return str
}

func envsFromConfig(cfg *config.Config) map[string]bool {
	envs := map[string]bool{}
	for env, _ := range cfg.Topology {
		envs[env] = true
	}
	return envs
}
