package tenantutils

import (
	"context"
	"net/url"
)

// query parameter for the default rule
// for requests generated by oauth2 proxy and meant for oauth2 proxy (self-redirects), we'll inject following query paramter containing tenantId
const DefaultTenantIdQueryParam = "tenant-id"

type contextKey string

const tenantIdKey contextKey = "tenantId"

// extarcts tenantId stored in a context
// returns empty string if tenantId not found
func FromContext(ctx context.Context) string {
	// Context can be nil
	if ctx == nil {
		return ""
	}
	t, ok := ctx.Value(tenantIdKey).(string)
	if !ok {
		return ""
	}
	return t
}

// stores tenantId in the context's key value pair
func AppendToContext(ctx context.Context, tenantId string) context.Context {
	return context.WithValue(ctx, tenantIdKey, tenantId)
}

// injects tenant-id in a url
func InjectTenantId(tid string, uri string) string {
	//if empty tenant-id, no need to inject it since empty tenant-id is loaded when it's not found in the request
	if tid == "" {
		return uri
	}
	u, err := url.Parse(uri)
	if err != nil {
		return uri
	}
	q := u.Query()
	q.Set(DefaultTenantIdQueryParam, tid)
	u.RawQuery = q.Encode()
	return u.String()
}
