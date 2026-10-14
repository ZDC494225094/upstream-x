package service

import (
	"strings"
	"testing"

	"upstream-x/server/models"
)

func TestValidateAccountPayloadRejectsRemovedBrowserAuthModes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		payload AccountPayload
		want    string
	}{
		{
			name: "sub2api",
			payload: AccountPayload{
				Name:            "sub2api",
				Type:            models.UpstreamTypeSub2API,
				BaseURL:         "https://sub2api.example.com",
				Sub2APIAuthMode: "server_browser",
			},
			want: "invalid sub2api_auth_mode",
		},
		{
			name: "newapi",
			payload: AccountPayload{
				Name:           "newapi",
				Type:           models.UpstreamTypeNewAPI,
				BaseURL:        "https://newapi.example.com",
				NewAPIAuthMode: "server_browser",
			},
			want: "invalid newapi_auth_mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAccountPayload(tt.payload, validationOptions{RequireCredentials: true})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("validation error = %v, want %q", err, tt.want)
			}
		})
	}
}
