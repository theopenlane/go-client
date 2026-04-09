package openlane

import (
	"context"
	"encoding/json"

	"github.com/theopenlane/httpsling"
	"github.com/theopenlane/utils/rout"

	api "github.com/theopenlane/core/common/openapi"
)

// IntegrationProvidersResponse is the response listing available integration definitions
type IntegrationProvidersResponse struct {
	rout.Reply
	// Providers is the list of available integration provider definitions
	Providers json.RawMessage `json:"providers"`
}

// ListIntegrationProviders returns declarative metadata about available third-party integration definitions
func (s *APIv1) ListIntegrationProviders(ctx context.Context) (out *IntegrationProvidersResponse, err error) {
	resp, err := s.Requester.ReceiveWithContext(ctx, &out,
		httpsling.Get(v1Path("integrations/providers")))
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if !httpsling.IsSuccess(resp) {
		return nil, newRequestError(resp.StatusCode, out.Error)
	}

	return out, nil
}

// ConfigureIntegration stores non-OAuth credentials for a provider definition
func (s *APIv1) ConfigureIntegration(ctx context.Context, in *api.ConfigureIntegrationRequest) (out *api.ConfigureIntegrationResponse, err error) {
	resp, err := s.Requester.ReceiveWithContext(ctx, &out,
		httpsling.Post(v1Path("integrations/"+in.DefinitionID+"/config")),
		httpsling.Body(in))
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if !httpsling.IsSuccess(resp) {
		return nil, newRequestError(resp.StatusCode, out.Error)
	}

	return out, nil
}

// DisconnectIntegration executes the definition-driven teardown flow for an installed integration
func (s *APIv1) DisconnectIntegration(ctx context.Context, in *api.DisconnectIntegrationRequest) (out *api.DeleteIntegrationResponse, err error) {
	resp, err := s.Requester.ReceiveWithContext(ctx, &out,
		httpsling.Post(v1Path("integrations/"+in.IntegrationID+"/disconnect")))
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if !httpsling.IsSuccess(resp) {
		return nil, newRequestError(resp.StatusCode, out.Error)
	}

	return out, nil
}

// RunIntegrationOperation executes or queues a provider operation against an installed integration
func (s *APIv1) RunIntegrationOperation(ctx context.Context, in *api.RunIntegrationOperationRequest) (out *api.RunIntegrationOperationResponse, err error) {
	resp, err := s.Requester.ReceiveWithContext(ctx, &out,
		httpsling.Post(v1Path("integrations/"+in.IntegrationID+"/operations/run")),
		httpsling.Body(in.Body))
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if !httpsling.IsSuccess(resp) {
		return nil, newRequestError(resp.StatusCode, out.Error)
	}

	return out, nil
}
