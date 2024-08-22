package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	log "github.com/sirupsen/logrus"

	"github.com/arryved/app-ctrl/api/config"
	"github.com/arryved/app-ctrl/api/gce"
	"github.com/arryved/app-ctrl/api/rbac"
	"github.com/arryved/app-ctrl/api/secrets"
)

// This role is used as a hint; users with the role will be restricted to access-only in other tools, but app-control
// will allow them to do more (full CRUD)
const accessorRole = "roles/secretmanager.secretAccessor"

// This limits how many IAM requests can happen at once. This won't scale past a few hundred secrets, but caching isn't
// necessary at the outset
const listIamConcurrency = 32

// context key for passing the authentication claims with the request
const AuthnClaimsKey = "authnClaims"
const EnvKey = "env"
const ProjectIdKey = "projectId"
const ProjectNumberKey = "projectNumber"

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

// Web handler for the endpoint
func ConfiguredHandlerSecrets(cfg *config.Config) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var action config.Permission

		//
		// Initial validation

		// valid path form?
		urlElements := strings.Split(r.URL.String(), "/")
		if len(urlElements) < 3 {
			msg := fmt.Sprintf("invalid number of path elements in request")
			handleBadRequest(w, msg)
			return
		}

		// user authenticated?
		if !authenticated(cfg, r) {
			msg := fmt.Sprintf("user not authenticated")
			handleUnauthorized(w, msg)
			return
		}
		claims := getClaims(r)
		log.Debugf("claims=%v", claims)

		// determine action/permission being requested
		if r.Method == http.MethodGet && len(urlElements) == 3 {
			action = config.SecretsList
		}
		if r.Method == http.MethodGet && len(urlElements) == 4 {
			action = config.SecretsRead
		}
		if r.Method == http.MethodPost && len(urlElements) == 3 {
			action = config.SecretsCreate
		}
		if r.Method == http.MethodPatch && len(urlElements) == 4 {
			action = config.SecretsUpdate
		}
		if r.Method == http.MethodDelete && len(urlElements) == 4 {
			action = config.SecretsDelete
		}

		// valid env?
		env := urlElements[2]
		envMap := envsFromConfig(cfg)
		_, ok := envMap[env]
		if !ok {
			msg := fmt.Sprintf("requested env=%s not supported by this instance", env)
			log.Info(msg)
			handleBadRequest(w, msg)
			return
		}

		// can get project metadata?
		projectId := gce.ProjectMap[env]
		projectNumber, err := gce.GetProjectNumber(projectId)
		if err != nil {
			log.Errorf("error getting a project number: err=%s", err.Error())
			msg := fmt.Errorf("error listing secrets; have the app administrator check the logs")
			handleInternalServerError(w, msg)
			return
		}

		// extend request context with the authenticated principal, env and GCP project id and number attached
		ctx := context.WithValue(r.Context(), AuthnClaimsKey, claims)
		ctx = context.WithValue(ctx, EnvKey, env)
		ctx = context.WithValue(ctx, ProjectIdKey, projectId)
		ctx = context.WithValue(ctx, ProjectNumberKey, projectNumber)
		r = r.WithContext(ctx)

		// get a secret client to inject
		client, err := secretmanager.NewClient(ctx)
		if err != nil {
			log.Errorf("error getting a secret client: err=%s", err.Error())
			msg := fmt.Errorf("error listing secrets; have the app administrator check the logs")
			handleInternalServerError(w, msg)
			return
		}
		defer client.Close()

		// authorization checks for read/create/update/delete
		principalUrn := config.PrincipalUrn(fmt.Sprintf("urn:arryved:user:%s", claims["email"]))
		var secretId string
		if len(urlElements) == 3 {
			secretId = ""
		} else {
			secretId = urlElements[3]
		}
		secretUrn := fmt.Sprintf("urn:arryved:secret:%s", secretId)
		if err := rbac.Authorized(r.Context(), cfg, client, principalUrn, action, secretUrn); err != nil {
			if err != nil && strings.Contains(err.Error(), "NotFound") {
				// capture the 404 case
				log.Infof("when acting on secret: err=%s", err.Error())
				msg := fmt.Sprintf("error acting on secret; could not find it")
				handleNotFound(w, msg)
				return
			}
			log.Infof("user not authorized for secrets action err=%s", err.Error())
			msg := fmt.Sprintf("user not authorized for secrets action")
			handleForbidden(w, msg)
			return
		}
		log.Debugf("Authorization granted for principal=%v, action=Deploy, app=%v", principalUrn, secretUrn)

		// dispatch to routine appropriate for action
		if action == config.SecretsList {
			SecretsList(cfg, client, w, r, secretId, projectNumber)
			return
		}
		if action == config.SecretsRead {
			SecretsRead(cfg, client, w, r, secretId, projectNumber)
			return
		}
		if action == config.SecretsCreate {
			SecretsCreate(cfg, client, w, r, secretId, projectNumber)
			return
		}
		if action == config.SecretsUpdate {
			SecretsUpdate(cfg, client, w, r, secretId, projectNumber)
			return
		}
		if action == config.SecretsDelete {
			SecretsDelete(cfg, client, w, r, secretId, projectNumber)
			return
		}
		// catch-all failure for unsupported method/uri combos
		msg := fmt.Sprintf("%s and/or uri not valid for this endpoint", r.Method)
		handleMethodNotAllowed(w, msg)
		return
	}
}

// LIST secrets
func SecretsList(cfg *config.Config, client secrets.SecretManagerClient, w http.ResponseWriter, r *http.Request, secretId, projectNumber string) {
	secretEntries, err := secrets.SecretList(r.Context(), client, projectNumber)
	if err != nil {
		log.Errorf("error listing secrets: err=%s", err.Error())
		msg := fmt.Errorf("error listing secrets; have the app administrator check the logs")
		handleInternalServerError(w, msg)
		return
	}
	responseBody, err := json.Marshal(secretEntries)
	if err != nil {
		log.Errorf("error marshalling statuses: err=%s", err.Error())
		msg := fmt.Errorf("error listing secrets; have the app administrator check the logs")
		handleInternalServerError(w, msg)
		return
	}
	httpStatus := http.StatusOK
	log.Infof("%s %s %s %d", r.RemoteAddr, r.Method, r.URL, httpStatus)
	log.Infof("secretEntries=%v", secretEntries)
	w.WriteHeader(httpStatus)
	w.Write(responseBody)
	return
}

// READ secret by id
func SecretsRead(cfg *config.Config, client secrets.SecretManagerClient, w http.ResponseWriter, r *http.Request, secretId, projectNumber string) {
	value, err := secrets.SecretRead(r.Context(), client, projectNumber, secretId)
	if err != nil {
		log.Errorf("error getting secretId=%s: err=%s", secretId, err.Error())
		msg := fmt.Errorf("error getting secret; have the app administrator check the logs")
		handleInternalServerError(w, msg)
		return
	}
	// marshaling the bytes yields bare json string of base64-encoded bytes, which is what we want since
	// secret data can be binary
	responseBody, err := json.Marshal(value)
	if err != nil {
		log.Errorf("error marshalling secret: err=%s", err.Error())
		msg := fmt.Errorf("error getting secret; have the app administrator check the logs")
		handleInternalServerError(w, msg)
		return
	}
	httpStatus := http.StatusOK
	log.Infof("%s %s %s %d", r.RemoteAddr, r.Method, r.URL, httpStatus)
	w.WriteHeader(httpStatus)
	w.Write(responseBody)
	return
}

// CREATE secret
func SecretsCreate(cfg *config.Config, client secrets.SecretManagerClient, w http.ResponseWriter, r *http.Request, secretId, projectNumber string) {
	// parse the POST json request body (via r *http.Request) into a SecretRequest
	var requestBody SecretRequest
	err := json.NewDecoder(r.Body).Decode(&requestBody)
	if err != nil {
		logMsg := fmt.Sprintf("could not decode request body: url=%v, err=%s", r.URL, err.Error())
		log.Infof(logMsg)
		msg := fmt.Sprintf("could not decode request body; err=%s", err.Error())
		handleBadRequest(w, msg)
		return
	}

	// set ownerUser to the authenticated principal; ignores whatever the user sent, if anything
	claims := r.Context().Value(AuthnClaimsKey).(map[string]interface{})
	if claims == nil {
		msg := fmt.Errorf("error creating secret; authn claims are missing")
		handleInternalServerError(w, msg)
		return
	}
	requestBody.OwnerUser = claims["email"].(string)

	// validate
	err = SecretRequestCreateValidate(requestBody)

	if err != nil {
		logMsg := fmt.Sprintf("invalid request body: url=%v, err=%s", r.URL, err.Error())
		log.Infof(logMsg)
		msg := fmt.Sprintf("failed to decode and/or validate body; err=%s", err.Error())
		handleBadRequest(w, msg)
		return
	}
	valueBytes, _ := base64.StdEncoding.DecodeString(requestBody.Value)
	err = secrets.SecretCreate(r.Context(), client, projectNumber, requestBody.Id, valueBytes)
	if err != nil && strings.Contains(err.Error(), "AlreadyExists") {
		// already exists case should 409
		log.Infof("error creating secret: err=%s", err.Error())
		msg := fmt.Sprintf("error creating secret; already exists")
		handleConflict(w, msg)
		return
	}
	if err != nil {
		log.Errorf("error creating secret: err=%s", err.Error())
		msg := fmt.Errorf("error creating secret; have the app administrator check the logs")
		handleInternalServerError(w, msg)
		return
	}

	// secret was created, so set the resource permissions
	secretName := fmt.Sprintf("projects/%s/secrets/%s", projectNumber, requestBody.Id)
	err = secrets.SecretIamSet(r.Context(), client, secretName, requestBody.OwnerUser, requestBody.OwnerGroup, cfg.SecretsServiceAccounts)
	if err != nil {
		log.Warnf("error setting permissions on secret err=%s", err.Error())
	}

	responseBody, err := json.Marshal(secrets.SecretEntry{
		Urn:            fmt.Sprintf("urn:arryved:secret:%s", requestBody.Id),
		OwnerGroup:     requestBody.OwnerGroup,
		OwnerUser:      requestBody.OwnerUser,
		CreatedEpochNs: time.Now().UnixNano(),
	})
	if err != nil {
		log.Errorf("error marshalling secret entry: err=%s", err.Error())
		msg := fmt.Errorf("error marshalling secret; have the app administrator check the logs")
		handleInternalServerError(w, msg)
		return
	}
	httpStatus := http.StatusOK
	log.Infof("%s %s %s %d", r.RemoteAddr, r.Method, r.URL, httpStatus)
	w.WriteHeader(httpStatus)
	w.Write(responseBody)
	return
}

// DELETE secret
func SecretsDelete(cfg *config.Config, client secrets.SecretManagerClient, w http.ResponseWriter, r *http.Request, secretId, projectNumber string) {
	err := secrets.SecretDelete(r.Context(), client, projectNumber, secretId)
	if err != nil {
		log.Errorf("error deleting secret: err=%s", err.Error())
		msg := fmt.Errorf("error deleting secret; have the app administrator check the logs")
		handleInternalServerError(w, msg)
		return
	}
	httpStatus := http.StatusNoContent // 204
	log.Infof("%s %s %s %d", r.RemoteAddr, r.Method, r.URL, httpStatus)
	w.WriteHeader(httpStatus)
	return
}

// UPDATE secret
func SecretsUpdate(cfg *config.Config, client secrets.SecretManagerClient, w http.ResponseWriter, r *http.Request, secretId, projectNumber string) {
	// parse the PATCH json request body (via r *http.Request) into a SecretRequest
	var requestBody SecretRequest
	err := json.NewDecoder(r.Body).Decode(&requestBody)
	validateErr := SecretRequestUpdateValidate(requestBody)
	if err != nil || validateErr != nil {
		logMsg := fmt.Sprintf("invalid request body: url=%v, err=%v, validateErr=%s", r.URL, err, validateErr)
		log.Infof(logMsg)
		msg := fmt.Sprintf("failed to decode and/or validate body; validatErr=%s", validateErr.Error())
		handleBadRequest(w, msg)
		return
	}
	valueBytes, _ := base64.StdEncoding.DecodeString(requestBody.Value)
	err = secrets.SecretUpdate(r.Context(), client, projectNumber, secretId, valueBytes)
	if err != nil && strings.Contains(err.Error(), "NotFound") {
		// capture the 404 case
		log.Errorf("error updating secret: err=%s", err.Error())
		msg := fmt.Sprintf("error updating secret; could not find it")
		handleNotFound(w, msg)
		return
	}
	if err != nil {
		log.Errorf("error updating secret: err=%s", err.Error())
		msg := fmt.Errorf("error updating secret; have the app administrator check the logs")
		handleInternalServerError(w, msg)
		return
	}
	httpStatus := http.StatusNoContent // 204
	log.Infof("%s %s %s %d", r.RemoteAddr, r.Method, r.URL, httpStatus)
	w.WriteHeader(httpStatus)
	return
}

func envsFromConfig(cfg *config.Config) map[string]bool {
	envs := map[string]bool{}
	for env, _ := range cfg.Topology {
		envs[env] = true
	}
	return envs
}
