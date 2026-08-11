package productremote

import (
	"context"
	"fmt"
	"net/url"

	stores "github.com/v8tix/beecore-http/messages/stores/v1"
	"github.com/v8tix/kawa"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
)

// FindProduct mirrors BaseRepositoryImpl.FindProduct: GET
// stores/products/{id} against the beecore-stores service.
func (c *Client) FindProduct(ctx context.Context, id, token string) (domain.Product, error) {
	productURL := fmt.Sprintf("%s/%s/%s/%s", c.cfg.Integration.V1.StoresURL, "stores", "products", id)

	call := kawa.NewCall[kawa.NoReq, stores.GetProductEnvV1Res](c.cfg.Web.HTTPClient, kawa.Get, productURL).
		WithHeaders(buildAuthHeader(token)).
		WithDeadline(c.deadline()).
		WithRetryPolicy(c.retryPolicy())

	env, err := call.DoWithRetry(ctx, nil)
	if err != nil {
		return domain.Product{}, err
	}

	if env.Body == nil {
		return domain.Product{}, nil
	}

	return toDomainProduct(env.Body.Product), nil
}

// FindPaginatedProducts mirrors BaseRepositoryImpl.FindPaginatedProducts:
// queries the Stores API's full-text search-count endpoint
// (products/search/count) and its search endpoint (products/search)
// separately, pairing the results into a single domain.ProductPage. page
// is zero-based (0,1,2,...).
func (c *Client) FindPaginatedProducts(ctx context.Context, searchTerm string, page int64, token string) (domain.ProductPage, error) {
	countURL := fmt.Sprintf("%s/%s", c.cfg.Integration.V1.StoresURL, "products/search/count")
	cq := url.Values{}
	cq.Set("q", searchTerm)
	countURL = countURL + "?" + cq.Encode()

	countCall := kawa.NewCall[kawa.NoReq, stores.GetProductsSearchCountEnvV1Res](c.cfg.Web.HTTPClient, kawa.Get, countURL).
		WithHeaders(buildAuthHeader(token)).
		WithDeadline(c.deadline()).
		WithRetryPolicy(c.retryPolicy())

	countEnv, err := countCall.DoWithRetry(ctx, nil)
	if err != nil {
		return domain.ProductPage{}, err
	}

	var total int64
	if countEnv.Body != nil {
		total = int64(countEnv.Body.Total)
	}

	searchURL := fmt.Sprintf("%s/%s", c.cfg.Integration.V1.StoresURL, "products/search")
	sq := url.Values{}
	sq.Set("q", searchTerm)
	sq.Set("page", fmt.Sprintf("%d", page))
	searchURL = searchURL + "?" + sq.Encode()

	searchCall := kawa.NewCall[kawa.NoReq, stores.GetProductsEnvV1Res](c.cfg.Web.HTTPClient, kawa.Get, searchURL).
		WithHeaders(buildAuthHeader(token)).
		WithDeadline(c.deadline()).
		WithRetryPolicy(c.retryPolicy())

	searchEnv, err := searchCall.DoWithRetry(ctx, nil)
	if err != nil {
		return domain.ProductPage{}, err
	}

	result := domain.ProductPage{Total: total}
	if searchEnv.Body != nil {
		result.Products = make([]domain.Product, 0, len(searchEnv.Body.Products))
		for _, p := range searchEnv.Body.Products {
			result.Products = append(result.Products, toDomainProduct(p))
		}
	}

	return result, nil
}
