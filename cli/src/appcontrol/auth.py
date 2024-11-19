from arpy_sso.auth import CLIAuthenticator


def token():
    metadata = {
      "web": {
        "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
        "auth_uri": "https://accounts.google.com/o/oauth2/auth",
        "client_id": "TODO",
        # this is not a secret in this context
        # see https://googleapis.github.io/google-api-python-client/docs/oauth-installed.html
        "client_secret": "TODO",
        "javascript_origins": ["http://localhost:8989"],
        "project_id": "arryved-tools",
        "redirect_uris": ["http://localhost:8989"],
        "token_uri": "https://oauth2.googleapis.com/token",
      }
    }
    authenticator = CLIAuthenticator(client_metadata=metadata)
    return authenticator.auth()
