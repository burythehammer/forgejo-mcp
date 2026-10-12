// SPDX-License-Identifier: GPL-3.0-or-later

package operation

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/oauthrs"
	"go.uber.org/zap"
)

// mintedTokenKey carries the Forgejo token minted for a request, from the
// authentication layer to requestTokenContextFunc. It is unexported so that
// nothing outside this package can place a credential in a request context.
type mintedTokenKey struct{}

// protectedResourceMetadata is the RFC 9728 document a resource server publishes
// so a client can find its authorization server.
type protectedResourceMetadata struct {
	Resource               string   `json:"resource"`
	AuthorizationServers   []string `json:"authorization_servers"`
	BearerMethodsSupported []string `json:"bearer_methods_supported"`
	ScopesSupported        []string `json:"scopes_supported,omitempty"`
}

// maxAudienceClaimLength bounds the Forgejo audience taken from a token.
const maxAudienceClaimLength = 256

// metadataDocument returns the protected resource metadata document for rs.
func (rs *resourceServer) metadataDocument() ([]byte, error) {
	doc, err := json.Marshal(protectedResourceMetadata{
		Resource:               rs.resource,
		AuthorizationServers:   []string{rs.authServer},
		BearerMethodsSupported: []string{"header"},
		ScopesSupported:        rs.scopes,
	})
	if err != nil {
		return nil, fmt.Errorf("encode protected resource metadata: %w", err)
	}
	return doc, nil
}

// bearerChallenge is the WWW-Authenticate value of every 401, without the error
// parameter. Its inputs are validated at startup, so no quoting is needed.
func bearerChallenge(metadataURL string, scopes []string) string {
	challenge := `Bearer resource_metadata="` + metadataURL + `"`
	if len(scopes) > 0 {
		challenge += `, scope="` + strings.Join(scopes, " ") + `"`
	}
	return challenge
}

// resourceAuth is what the resource-server port returns for a request it
// accepts: the Forgejo credential minted for it, and the identity and audience
// it was minted for, for the debug log.
type resourceAuth struct {
	credential credential
	subject    string
	audience   string
}

// resourceRefusal is the port's refusal: the answer the transport writes and the
// debug line it logs. It is an error so the port can return it with the
// credential. A value that is not a resourceRefusal is a server-side failure.
type resourceRefusal struct {
	status    int
	challenge string // WWW-Authenticate, or "" for none
	body      string // plain-text body, or "" for none
	logMsg    string
	logFields []zap.Field
}

func (e *resourceRefusal) Error() string { return e.logMsg }

// writeTo writes the refusal's answer. A refusal with no body writes no body
// and sets no content type.
func (e *resourceRefusal) writeTo(w http.ResponseWriter) {
	if e.challenge != "" {
		w.Header().Set("WWW-Authenticate", e.challenge)
	}
	if e.body != "" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	}
	w.WriteHeader(e.status)
	if e.body != "" {
		_, _ = io.WriteString(w, e.body)
	}
}

// authenticateResourceServer is the authentication port in resource-server
// mode. It validates the caller's access token, derives the Forgejo audience and
// mints the Forgejo token for the request. challenge is the WWW-Authenticate
// value of a 401 for this server.
//
// The caller's own token goes no further than this function. A refusal is a
// *resourceRefusal; any other error is a failure to mint, which the transport
// answers as a server error.
func authenticateResourceServer(r *http.Request, rs *resourceServer, challenge string) (resourceAuth, error) {
	claims, err := rs.validator.ValidateAuthorization(r.Context(), r.Header.Get("Authorization"))
	if err == nil && claims.Subject == "" {
		err = fmt.Errorf("%w: the token has no sub", oauthrs.ErrInvalidToken)
	}
	if err != nil {
		value := challenge
		if errors.Is(err, oauthrs.ErrInvalidToken) {
			value += `, error="invalid_token"`
		}
		return resourceAuth{}, &resourceRefusal{
			status:    http.StatusUnauthorized,
			challenge: value,
			logMsg:    "Refused a request to the MCP endpoint",
			logFields: []zap.Field{
				log.StringField("reason", err.Error()),
				log.StringField("path", truncateForLog(r.URL.Path)),
			},
		}
	}

	audience, ok := usableAudience(claims.Raw[rs.audienceClaim])
	if !ok {
		return resourceAuth{}, &resourceRefusal{
			// No WWW-Authenticate challenge, deliberately (design D4). The token
			// is valid; what is missing is data at the identity provider, not
			// authorization or scope. A challenge, with or without
			// error="insufficient_scope", invites a spec-following client to
			// re-authorize or step up its scopes, and that loop cannot succeed.
			status: http.StatusForbidden,
			body: fmt.Sprintf(
				"Forbidden: the access token carries no usable %q claim. It must hold the audience of your Forgejo "+
					"Authorized Integration for this server.\n", rs.audienceClaim),
			logMsg: "Refused a token without a usable Forgejo audience claim",
			logFields: []zap.Field{
				log.StringField("claim", rs.audienceClaim),
				log.StringField("sub", truncateForLog(claims.Subject)),
			},
		}
	}

	token, err := rs.issuer.Mint(claims.Subject, audience, time.Now())
	if err != nil {
		return resourceAuth{}, fmt.Errorf("mint forgejo token: %w", err)
	}
	return resourceAuth{
		credential: credential(token),
		subject:    claims.Subject,
		audience:   audience,
	}, nil
}

// usableAudience accepts a single non-empty string of bounded length with no
// whitespace or control characters. Forgejo requires exactly one audience.
func usableAudience(v any) (string, bool) {
	s, ok := v.(string)
	if !ok || s == "" || len(s) > maxAudienceClaimLength {
		return "", false
	}
	for _, r := range s {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return "", false
		}
	}
	return s, true
}
