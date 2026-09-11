package client

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

// ConnectionEnvironment maps user attribute values to an alternate connection.
//
// This is how physical tenant isolation works: a signed embed URL carries a
// user attribute value, and Omni routes queries to the connection whose
// environment lists that value.
//
// The API has no GET for connection environments, so the provider cannot detect
// drift on them. See ReadLimitation in the resource documentation.
type ConnectionEnvironment struct {
	ID                  string   `json:"id"`
	BaseConnectionID    string   `json:"baseConnectionId"`
	ConnectionID        string   `json:"connectionId"`
	UserAttributeValues []string `json:"userAttributeValues"`
}

type createConnectionEnvironmentsRequest struct {
	BaseConnectionID         string   `json:"baseConnectionId"`
	EnvironmentConnectionIDs []string `json:"environmentConnectionIds"`
}

type createConnectionEnvironmentsResponse struct {
	ConnectionEnvironments []ConnectionEnvironment `json:"connectionEnvironments"`
}

type updateConnectionEnvironmentRequest struct {
	UserAttributeValues []string `json:"userAttributeValues"`
}

// CreateConnectionEnvironment attaches one connection to a base connection as
// an environment, and returns the environment it created.
//
// The endpoint accepts a list, but the provider models one environment per
// resource so that Terraform can address them individually.
func (c *Client) CreateConnectionEnvironment(ctx context.Context, baseConnectionID, environmentConnectionID string) (*ConnectionEnvironment, error) {
	var out createConnectionEnvironmentsResponse
	err := c.Post(ctx, "/v1/connection-environments", createConnectionEnvironmentsRequest{
		BaseConnectionID:         baseConnectionID,
		EnvironmentConnectionIDs: []string{environmentConnectionID},
	}, &out)
	if err != nil {
		return nil, err
	}

	for i := range out.ConnectionEnvironments {
		if out.ConnectionEnvironments[i].ConnectionID == environmentConnectionID {
			return &out.ConnectionEnvironments[i], nil
		}
	}
	if len(out.ConnectionEnvironments) == 1 {
		return &out.ConnectionEnvironments[0], nil
	}
	return nil, fmt.Errorf("created a connection environment but the response did not identify it: %+v", out.ConnectionEnvironments)
}

// SetConnectionEnvironmentValues sets which user attribute values route to this
// environment.
func (c *Client) SetConnectionEnvironmentValues(ctx context.Context, id string, values []string) error {
	if values == nil {
		values = []string{}
	}
	return c.Put(ctx, "/v1/connection-environments/"+url.PathEscape(id), updateConnectionEnvironmentRequest{
		UserAttributeValues: values,
	}, nil)
}

// DeleteConnectionEnvironment detaches an environment from its base connection.
func (c *Client) DeleteConnectionEnvironment(ctx context.Context, id string) error {
	return c.Delete(ctx, "/v1/connection-environments/"+url.PathEscape(id), nil)
}

// ConnectionSchedule is a schema refresh schedule on a connection.
type ConnectionSchedule struct {
	ScheduleID   string  `json:"scheduleId"`
	ConnectionID string  `json:"connectionId"`
	Schedule     string  `json:"schedule"`
	Timezone     string  `json:"timezone"`
	HardRefresh  bool    `json:"hardRefresh"`
	Description  string  `json:"description"`
	DisabledAt   *string `json:"disabledAt"`
	CreatedAt    string  `json:"createdAt"`
	UpdatedAt    string  `json:"updatedAt"`
}

// ConnectionScheduleInput is the create and update body.
type ConnectionScheduleInput struct {
	// Schedule is an AWS EventBridge cron expression with six fields:
	// minute hour day-of-month month day-of-week year.
	Schedule    string `json:"schedule"`
	Timezone    string `json:"timezone"`
	HardRefresh *bool  `json:"hardRefresh,omitempty"`
}

type listConnectionSchedulesResponse struct {
	Schedules []ConnectionSchedule `json:"schedules"`
}

func connectionSchedulesPath(connectionID string) string {
	return "/v1/connections/" + url.PathEscape(connectionID) + "/schedules"
}

func connectionSchedulePath(connectionID, scheduleID string) string {
	return connectionSchedulesPath(connectionID) + "/" + url.PathEscape(scheduleID)
}

// CreateConnectionSchedule adds a schema refresh schedule to a connection.
//
// A connection's sub-resources are not addressable the instant the connection
// is created: this endpoint can return 404 for a connection that GET
// /v1/connections/{id} already serves. Terraform creates the two back to back,
// so the 404 is retried rather than surfaced as a missing connection.
func (c *Client) CreateConnectionSchedule(ctx context.Context, connectionID string, in ConnectionScheduleInput) (*ConnectionSchedule, error) {
	const attempts = 5

	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt*3) * time.Second):
			}
		}

		var out ConnectionSchedule
		err := c.Post(ctx, connectionSchedulesPath(connectionID), in, &out)
		if err == nil {
			return &out, nil
		}
		lastErr = err
		// 404 is the propagation delay. 429 means the inner backoff in Do ran
		// out of attempts, which the outer loop can absorb by waiting longer.
		if !IsNotFound(err) && !IsRateLimited(err) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("creating a schedule on connection %s failed after %d attempts: %w", connectionID, attempts, lastErr)
}

// GetConnectionSchedule fetches one schedule.
func (c *Client) GetConnectionSchedule(ctx context.Context, connectionID, scheduleID string) (*ConnectionSchedule, error) {
	var out ConnectionSchedule
	if err := c.Get(ctx, connectionSchedulePath(connectionID, scheduleID), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateConnectionSchedule changes the cron expression, timezone or refresh mode.
func (c *Client) UpdateConnectionSchedule(ctx context.Context, connectionID, scheduleID string, in ConnectionScheduleInput) (*ConnectionSchedule, error) {
	var out ConnectionSchedule
	if err := c.Put(ctx, connectionSchedulePath(connectionID, scheduleID), in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteConnectionSchedule removes a schedule.
func (c *Client) DeleteConnectionSchedule(ctx context.Context, connectionID, scheduleID string) error {
	return c.Delete(ctx, connectionSchedulePath(connectionID, scheduleID), nil)
}

// ListConnectionSchedules returns every schedule on a connection.
func (c *Client) ListConnectionSchedules(ctx context.Context, connectionID string) ([]ConnectionSchedule, error) {
	var out listConnectionSchedulesResponse
	if err := c.Get(ctx, connectionSchedulesPath(connectionID), nil, &out); err != nil {
		return nil, err
	}
	return out.Schedules, nil
}
