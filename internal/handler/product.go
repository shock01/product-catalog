package handler

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/shock01/product-catalog/internal/domain"
	"github.com/shock01/product-catalog/internal/gen"
	"github.com/shock01/product-catalog/internal/usecase"
)

// ProductHandler implements gen.StrictServerInterface.
// It is pure HTTP infrastructure: translate request → use-case, result → response.
type ProductHandler struct {
	commands usecase.ProductCommands
	queries  usecase.ProductQueries
}

func New(commands usecase.ProductCommands, queries usecase.ProductQueries) *ProductHandler {
	return &ProductHandler{commands: commands, queries: queries}
}

// ---------------------------------------------------------------------------
// Health
// ---------------------------------------------------------------------------

func (h *ProductHandler) HealthLiveness(_ context.Context, _ gen.HealthLivenessRequestObject) (gen.HealthLivenessResponseObject, error) {
	return gen.HealthLiveness200Response{}, nil
}

func (h *ProductHandler) HealthReadiness(_ context.Context, _ gen.HealthReadinessRequestObject) (gen.HealthReadinessResponseObject, error) {
	// TODO: check db + cache connectivity
	return gen.HealthReadiness200Response{}, nil
}

// ---------------------------------------------------------------------------
// catalog/product — queries
// ---------------------------------------------------------------------------

func (h *ProductHandler) GetProduct(ctx context.Context, req gen.GetProductRequestObject) (gen.GetProductResponseObject, error) {
	p, etag, err := h.queries.GetByID(ctx, req.Id)
	if err != nil {
		return mapGetError(err), nil
	}
	resp := gen.GetProduct200JSONResponse{
		Headers: gen.GetProduct200ResponseHeaders{ETag: &etag},
		Body: gen.ProductResponse{
			Data:  domainToProduct(p),
			Meta:  gen.SingleMeta{Version: p.Version, Etag: etag},
			Links: productLinks(p.ID),
		},
	}
	return resp, nil
}

// ---------------------------------------------------------------------------
// catalog/product — commands
// ---------------------------------------------------------------------------

func (h *ProductHandler) CreateProduct(ctx context.Context, req gen.CreateProductRequestObject) (gen.CreateProductResponseObject, error) {
	p := createRequestToDomain(req.Body)
	if err := h.commands.Create(ctx, p); err != nil {
		return mapCreateError(err), nil
	}
	loc := "/catalog/product/v1/" + p.ID.String()
	return gen.CreateProduct202JSONResponse{
		Headers: gen.CreateProduct202ResponseHeaders{Location: &loc},
		Body:    gen.CommandResponse{Links: productLinks(p.ID)},
	}, nil
}

func (h *ProductHandler) ReplaceProduct(ctx context.Context, req gen.ReplaceProductRequestObject) (gen.ReplaceProductResponseObject, error) {
	p := updateRequestToDomain(req.Id, req.Body)
	if err := h.commands.Update(ctx, p); err != nil {
		return mapReplaceError(err), nil
	}
	return gen.ReplaceProduct202JSONResponse{
		Body: gen.CommandResponse{Links: productLinks(req.Id)},
	}, nil
}

func (h *ProductHandler) UpdateProduct(ctx context.Context, req gen.UpdateProductRequestObject) (gen.UpdateProductResponseObject, error) {
	p := patchRequestToDomain(req.Id, req.Body)
	if err := h.commands.Patch(ctx, p); err != nil {
		return mapUpdateError(err), nil
	}
	return gen.UpdateProduct202JSONResponse{
		Body: gen.CommandResponse{Links: productLinks(req.Id)},
	}, nil
}

func (h *ProductHandler) DeleteProduct(ctx context.Context, req gen.DeleteProductRequestObject) (gen.DeleteProductResponseObject, error) {
	// TODO: parse version from If-Match header
	if err := h.commands.Delete(ctx, req.Id, 0); err != nil {
		return mapDeleteError(err), nil
	}
	return gen.DeleteProduct202JSONResponse(gen.CommandResponse{Links: searchLinks()}), nil
}

// ---------------------------------------------------------------------------
// catalog/search
// ---------------------------------------------------------------------------

func (h *ProductHandler) SearchProducts(_ context.Context, _ gen.SearchProductsRequestObject) (gen.SearchProductsResponseObject, error) {
	// TODO: delegate to OpenSearch query use-case
	return gen.SearchProducts200JSONResponse(gen.SearchResponse{Meta: gen.PaginationMeta{Total: 0}}), nil
}

// ---------------------------------------------------------------------------
// Mapping helpers — domain → gen types
// ---------------------------------------------------------------------------

func domainToProduct(p *domain.Product) gen.Product {
	// TODO: full field mapping
	v := p.Version
	return gen.Product{
		Id:      p.ID,
		Status:  gen.ProductStatus(p.Status),
		Version: &v,
	}
}

func productLinks(id uuid.UUID) []gen.Link {
	base := "/catalog/product/v1/" + id.String()
	return []gen.Link{
		{Method: gen.GET, Href: base, Rel: gen.Self},
		{Method: gen.PUT, Href: base, Rel: gen.Replace},
		{Method: gen.PATCH, Href: base, Rel: gen.Update},
		{Method: gen.DELETE, Href: base, Rel: gen.Delete},
		{Method: gen.GET, Href: "/catalog/search/v1", Rel: gen.Search},
	}
}

func searchLinks() []gen.Link {
	return []gen.Link{{Method: gen.GET, Href: "/catalog/search/v1", Rel: gen.Search}}
}

// ---------------------------------------------------------------------------
// Request → domain
// ---------------------------------------------------------------------------

func createRequestToDomain(_ *gen.CreateProductRequest) *domain.Product {
	// TODO: full mapping
	return &domain.Product{}
}

func updateRequestToDomain(id uuid.UUID, _ *gen.UpdateProductRequest) *domain.Product {
	return &domain.Product{ID: id}
}

func patchRequestToDomain(id uuid.UUID, _ *gen.PatchProductRequest) *domain.Product {
	return &domain.Product{ID: id}
}

// ---------------------------------------------------------------------------
// Error mapping
// ---------------------------------------------------------------------------

func errResp(code, msg string) gen.ErrorResponse {
	return gen.ErrorResponse{Error: struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}{Code: code, Message: msg}}
}

func mapGetError(err error) gen.GetProductResponseObject {
	if errors.Is(err, domain.ErrDeleted) {
		return gen.GetProduct410JSONResponse{GoneJSONResponse: gen.GoneJSONResponse(errResp("PRODUCT_DELETED", err.Error()))}
	}
	return gen.GetProduct404JSONResponse{NotFoundJSONResponse: gen.NotFoundJSONResponse(errResp("PRODUCT_NOT_FOUND", err.Error()))}
}

func mapCreateError(err error) gen.CreateProductResponseObject {
	switch {
	case errors.Is(err, domain.ErrSKUConflict):
		return gen.CreateProduct409JSONResponse{ConflictJSONResponse: gen.ConflictJSONResponse(errResp("SKU_CONFLICT", err.Error()))}
	case errors.Is(err, domain.ErrInvariantViolation):
		return gen.CreateProduct422JSONResponse{UnprocessableEntityJSONResponse: gen.UnprocessableEntityJSONResponse(errResp("INVARIANT_VIOLATION", err.Error()))}
	}
	return gen.CreateProduct400JSONResponse{BadRequestJSONResponse: gen.BadRequestJSONResponse(errResp("VALIDATION_ERROR", err.Error()))}
}

func mapReplaceError(err error) gen.ReplaceProductResponseObject {
	switch {
	case errors.Is(err, domain.ErrVersionConflict):
		return gen.ReplaceProduct412JSONResponse{VersionConflictJSONResponse: gen.VersionConflictJSONResponse(errResp("VERSION_CONFLICT", err.Error()))}
	case errors.Is(err, domain.ErrNotFound):
		return gen.ReplaceProduct404JSONResponse{NotFoundJSONResponse: gen.NotFoundJSONResponse(errResp("PRODUCT_NOT_FOUND", err.Error()))}
	case errors.Is(err, domain.ErrSKUConflict):
		return gen.ReplaceProduct409JSONResponse{ConflictJSONResponse: gen.ConflictJSONResponse(errResp("SKU_CONFLICT", err.Error()))}
	}
	return gen.ReplaceProduct400JSONResponse{BadRequestJSONResponse: gen.BadRequestJSONResponse(errResp("VALIDATION_ERROR", err.Error()))}
}

func mapUpdateError(err error) gen.UpdateProductResponseObject {
	switch {
	case errors.Is(err, domain.ErrVersionConflict):
		return gen.UpdateProduct412JSONResponse{VersionConflictJSONResponse: gen.VersionConflictJSONResponse(errResp("VERSION_CONFLICT", err.Error()))}
	case errors.Is(err, domain.ErrNotFound):
		return gen.UpdateProduct404JSONResponse{NotFoundJSONResponse: gen.NotFoundJSONResponse(errResp("PRODUCT_NOT_FOUND", err.Error()))}
	}
	return gen.UpdateProduct400JSONResponse{BadRequestJSONResponse: gen.BadRequestJSONResponse(errResp("VALIDATION_ERROR", err.Error()))}
}

func mapDeleteError(err error) gen.DeleteProductResponseObject {
	switch {
	case errors.Is(err, domain.ErrVersionConflict):
		return gen.DeleteProduct412JSONResponse{VersionConflictJSONResponse: gen.VersionConflictJSONResponse(errResp("VERSION_CONFLICT", err.Error()))}
	case errors.Is(err, domain.ErrNotFound):
		return gen.DeleteProduct404JSONResponse{NotFoundJSONResponse: gen.NotFoundJSONResponse(errResp("PRODUCT_NOT_FOUND", err.Error()))}
	}
	return gen.DeleteProduct410JSONResponse{GoneJSONResponse: gen.GoneJSONResponse(errResp("PRODUCT_DELETED", err.Error()))}
}
