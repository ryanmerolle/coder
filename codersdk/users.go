package codersdk

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/xerrors"
)

// Me is used as a replacement for your own ID.
var Me = "me"

type UserStatus string

const (
	UserStatusActive    UserStatus = "active"
	UserStatusSuspended UserStatus = "suspended"
)

type LoginType string

const (
	LoginTypePassword LoginType = "password"
	LoginTypeGithub   LoginType = "github"
	LoginTypeOIDC     LoginType = "oidc"
)

type UsersRequest struct {
	Search string `json:"search,omitempty" typescript:"-"`
	// Filter users by status.
	Status UserStatus `json:"status,omitempty" typescript:"-"`
	// Filter users that have the given role.
	Role string `json:"role,omitempty" typescript:"-"`

	SearchQuery string `json:"q,omitempty"`
	Pagination
}

// User represents a user in Coder.
type User struct {
	ID              uuid.UUID   `json:"id" validate:"required" table:"id"`
	Username        string      `json:"username" validate:"required" table:"username"`
	Email           string      `json:"email" validate:"required" table:"email"`
	CreatedAt       time.Time   `json:"created_at" validate:"required" table:"created at"`
	Status          UserStatus  `json:"status" table:"status"`
	OrganizationIDs []uuid.UUID `json:"organization_ids"`
	Roles           []Role      `json:"roles"`
	AvatarURL       string      `json:"avatar_url"`
}

type APIKey struct {
	ID string `json:"id" validate:"required"`
	// NOTE: do not ever return the HashedSecret
	UserID          uuid.UUID `json:"user_id" validate:"required"`
	LastUsed        time.Time `json:"last_used" validate:"required"`
	ExpiresAt       time.Time `json:"expires_at" validate:"required"`
	CreatedAt       time.Time `json:"created_at" validate:"required"`
	UpdatedAt       time.Time `json:"updated_at" validate:"required"`
	LoginType       LoginType `json:"login_type" validate:"required"`
	LifetimeSeconds int64     `json:"lifetime_seconds" validate:"required"`
}

type CreateFirstUserRequest struct {
	Email            string `json:"email" validate:"required,email"`
	Username         string `json:"username" validate:"required,username"`
	Password         string `json:"password" validate:"required"`
	OrganizationName string `json:"organization" validate:"required,username"`
}

// CreateFirstUserResponse contains IDs for newly created user info.
type CreateFirstUserResponse struct {
	UserID         uuid.UUID `json:"user_id"`
	OrganizationID uuid.UUID `json:"organization_id"`
}

type CreateUserRequest struct {
	Email          string    `json:"email" validate:"required,email"`
	Username       string    `json:"username" validate:"required,username"`
	Password       string    `json:"password" validate:"required"`
	OrganizationID uuid.UUID `json:"organization_id" validate:"required"`
}

type UpdateUserProfileRequest struct {
	Username string `json:"username" validate:"required,username"`
}

type UpdateUserPasswordRequest struct {
	OldPassword string `json:"old_password" validate:""`
	Password    string `json:"password" validate:"required"`
}

type UpdateRoles struct {
	Roles []string `json:"roles" validate:""`
}

type UserRoles struct {
	Roles             []string               `json:"roles"`
	OrganizationRoles map[uuid.UUID][]string `json:"organization_roles"`
}

// LoginWithPasswordRequest enables callers to authenticate with email and password.
type LoginWithPasswordRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// LoginWithPasswordResponse contains a session token for the newly authenticated user.
type LoginWithPasswordResponse struct {
	SessionToken string `json:"session_token" validate:"required"`
}

// GenerateAPIKeyResponse contains an API key for a user.
type GenerateAPIKeyResponse struct {
	Key string `json:"key"`
}

type CreateOrganizationRequest struct {
	Name string `json:"name" validate:"required,username"`
}

// AuthMethods contains whether authentication types are enabled or not.
type AuthMethods struct {
	Password bool `json:"password"`
	Github   bool `json:"github"`
	OIDC     bool `json:"oidc"`
}

// HasFirstUser returns whether the first user has been created.
func (c *Client) HasFirstUser(ctx context.Context) (bool, error) {
	res, err := c.Request(ctx, http.MethodGet, "/api/v2/users/first", nil)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if res.StatusCode != http.StatusOK {
		return false, readBodyAsError(res)
	}
	return true, nil
}

// CreateFirstUser attempts to create the first user on a Coder deployment.
// This initial user has superadmin privileges. If >0 users exist, this request will fail.
func (c *Client) CreateFirstUser(ctx context.Context, req CreateFirstUserRequest) (CreateFirstUserResponse, error) {
	res, err := c.Request(ctx, http.MethodPost, "/api/v2/users/first", req)
	if err != nil {
		return CreateFirstUserResponse{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		return CreateFirstUserResponse{}, readBodyAsError(res)
	}
	var resp CreateFirstUserResponse
	return resp, json.NewDecoder(res.Body).Decode(&resp)
}

// CreateUser creates a new user.
func (c *Client) CreateUser(ctx context.Context, req CreateUserRequest) (User, error) {
	res, err := c.Request(ctx, http.MethodPost, "/api/v2/users", req)
	if err != nil {
		return User{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		return User{}, readBodyAsError(res)
	}
	var user User
	return user, json.NewDecoder(res.Body).Decode(&user)
}

// DeleteUser deletes a user.
func (c *Client) DeleteUser(ctx context.Context, id uuid.UUID) error {
	res, err := c.Request(ctx, http.MethodDelete, fmt.Sprintf("/api/v2/users/%s", id), nil)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return readBodyAsError(res)
	}
	return nil
}

// UpdateUserProfile enables callers to update profile information
func (c *Client) UpdateUserProfile(ctx context.Context, user string, req UpdateUserProfileRequest) (User, error) {
	res, err := c.Request(ctx, http.MethodPut, fmt.Sprintf("/api/v2/users/%s/profile", user), req)
	if err != nil {
		return User{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return User{}, readBodyAsError(res)
	}
	var resp User
	return resp, json.NewDecoder(res.Body).Decode(&resp)
}

// UpdateUserStatus sets the user status to the given status
func (c *Client) UpdateUserStatus(ctx context.Context, user string, status UserStatus) (User, error) {
	path := fmt.Sprintf("/api/v2/users/%s/status/", user)
	switch status {
	case UserStatusActive:
		path += "activate"
	case UserStatusSuspended:
		path += "suspend"
	default:
		return User{}, xerrors.Errorf("status %q is not supported", status)
	}

	res, err := c.Request(ctx, http.MethodPut, path, nil)
	if err != nil {
		return User{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return User{}, readBodyAsError(res)
	}

	var resp User
	return resp, json.NewDecoder(res.Body).Decode(&resp)
}

// UpdateUserPassword updates a user password.
// It calls PUT /users/{user}/password
func (c *Client) UpdateUserPassword(ctx context.Context, user string, req UpdateUserPasswordRequest) error {
	res, err := c.Request(ctx, http.MethodPut, fmt.Sprintf("/api/v2/users/%s/password", user), req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		return readBodyAsError(res)
	}
	return nil
}

// UpdateUserRoles grants the userID the specified roles.
// Include ALL roles the user has.
func (c *Client) UpdateUserRoles(ctx context.Context, user string, req UpdateRoles) (User, error) {
	res, err := c.Request(ctx, http.MethodPut, fmt.Sprintf("/api/v2/users/%s/roles", user), req)
	if err != nil {
		return User{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return User{}, readBodyAsError(res)
	}
	var resp User
	return resp, json.NewDecoder(res.Body).Decode(&resp)
}

// UpdateOrganizationMemberRoles grants the userID the specified roles in an org.
// Include ALL roles the user has.
func (c *Client) UpdateOrganizationMemberRoles(ctx context.Context, organizationID uuid.UUID, user string, req UpdateRoles) (OrganizationMember, error) {
	res, err := c.Request(ctx, http.MethodPut, fmt.Sprintf("/api/v2/organizations/%s/members/%s/roles", organizationID, user), req)
	if err != nil {
		return OrganizationMember{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return OrganizationMember{}, readBodyAsError(res)
	}
	var member OrganizationMember
	return member, json.NewDecoder(res.Body).Decode(&member)
}

// GetUserRoles returns all roles the user has
func (c *Client) GetUserRoles(ctx context.Context, user string) (UserRoles, error) {
	res, err := c.Request(ctx, http.MethodGet, fmt.Sprintf("/api/v2/users/%s/roles", user), nil)
	if err != nil {
		return UserRoles{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return UserRoles{}, readBodyAsError(res)
	}
	var roles UserRoles
	return roles, json.NewDecoder(res.Body).Decode(&roles)
}

// CreateAPIKey generates an API key for the user ID provided.
func (c *Client) CreateAPIKey(ctx context.Context, user string) (*GenerateAPIKeyResponse, error) {
	res, err := c.Request(ctx, http.MethodPost, fmt.Sprintf("/api/v2/users/%s/keys", user), nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode > http.StatusCreated {
		return nil, readBodyAsError(res)
	}
	apiKey := &GenerateAPIKeyResponse{}
	return apiKey, json.NewDecoder(res.Body).Decode(apiKey)
}

func (c *Client) GetAPIKey(ctx context.Context, user string, id string) (*APIKey, error) {
	res, err := c.Request(ctx, http.MethodGet, fmt.Sprintf("/api/v2/users/%s/keys/%s", user, id), nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode > http.StatusCreated {
		return nil, readBodyAsError(res)
	}
	apiKey := &APIKey{}
	return apiKey, json.NewDecoder(res.Body).Decode(apiKey)
}

// LoginWithPassword creates a session token authenticating with an email and password.
// Call `SetSessionToken()` to apply the newly acquired token to the client.
func (c *Client) LoginWithPassword(ctx context.Context, req LoginWithPasswordRequest) (LoginWithPasswordResponse, error) {
	res, err := c.Request(ctx, http.MethodPost, "/api/v2/users/login", req)
	if err != nil {
		return LoginWithPasswordResponse{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		return LoginWithPasswordResponse{}, readBodyAsError(res)
	}
	var resp LoginWithPasswordResponse
	err = json.NewDecoder(res.Body).Decode(&resp)
	if err != nil {
		return LoginWithPasswordResponse{}, err
	}
	return resp, nil
}

// Logout calls the /logout API
// Call `ClearSessionToken()` to clear the session token of the client.
func (c *Client) Logout(ctx context.Context) error {
	// Since `LoginWithPassword` doesn't actually set a SessionToken
	// (it requires a call to SetSessionToken), this is essentially a no-op
	res, err := c.Request(ctx, http.MethodPost, "/api/v2/users/logout", nil)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	return nil
}

// User returns a user for the ID/username provided.
func (c *Client) User(ctx context.Context, userIdent string) (User, error) {
	res, err := c.Request(ctx, http.MethodGet, fmt.Sprintf("/api/v2/users/%s", userIdent), nil)
	if err != nil {
		return User{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return User{}, readBodyAsError(res)
	}
	var user User
	return user, json.NewDecoder(res.Body).Decode(&user)
}

// Users returns all users according to the request parameters. If no parameters are set,
// the default behavior is to return all users in a single page.
func (c *Client) Users(ctx context.Context, req UsersRequest) ([]User, error) {
	res, err := c.Request(ctx, http.MethodGet, "/api/v2/users", nil,
		req.Pagination.asRequestOption(),
		func(r *http.Request) {
			q := r.URL.Query()
			var params []string
			if req.Search != "" {
				params = append(params, req.Search)
			}
			if req.Status != "" {
				params = append(params, "status:"+string(req.Status))
			}
			if req.Role != "" {
				params = append(params, "role:"+req.Role)
			}
			if req.SearchQuery != "" {
				params = append(params, req.SearchQuery)
			}
			q.Set("q", strings.Join(params, " "))
			r.URL.RawQuery = q.Encode()
		},
	)
	if err != nil {
		return []User{}, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return []User{}, readBodyAsError(res)
	}

	var users []User
	return users, json.NewDecoder(res.Body).Decode(&users)
}

// OrganizationsByUser returns all organizations the user is a member of.
func (c *Client) OrganizationsByUser(ctx context.Context, user string) ([]Organization, error) {
	res, err := c.Request(ctx, http.MethodGet, fmt.Sprintf("/api/v2/users/%s/organizations", user), nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode > http.StatusOK {
		return nil, readBodyAsError(res)
	}
	var orgs []Organization
	return orgs, json.NewDecoder(res.Body).Decode(&orgs)
}

func (c *Client) OrganizationByName(ctx context.Context, user string, name string) (Organization, error) {
	res, err := c.Request(ctx, http.MethodGet, fmt.Sprintf("/api/v2/users/%s/organizations/%s", user, name), nil)
	if err != nil {
		return Organization{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return Organization{}, readBodyAsError(res)
	}
	var org Organization
	return org, json.NewDecoder(res.Body).Decode(&org)
}

// CreateOrganization creates an organization and adds the provided user as an admin.
func (c *Client) CreateOrganization(ctx context.Context, req CreateOrganizationRequest) (Organization, error) {
	res, err := c.Request(ctx, http.MethodPost, "/api/v2/organizations", req)
	if err != nil {
		return Organization{}, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		return Organization{}, readBodyAsError(res)
	}

	var org Organization
	return org, json.NewDecoder(res.Body).Decode(&org)
}

// AuthMethods returns types of authentication available to the user.
func (c *Client) AuthMethods(ctx context.Context) (AuthMethods, error) {
	res, err := c.Request(ctx, http.MethodGet, "/api/v2/users/authmethods", nil)
	if err != nil {
		return AuthMethods{}, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return AuthMethods{}, readBodyAsError(res)
	}

	var userAuth AuthMethods
	return userAuth, json.NewDecoder(res.Body).Decode(&userAuth)
}
