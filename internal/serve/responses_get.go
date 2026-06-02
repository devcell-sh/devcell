package serve

import (
	"encoding/json"
	"net/http"
)

// NewResponseGetHandler returns an http.Handler for GET /v1/responses/{id}.
//
// Returns the current state of a background job as a ResponsesObject.
// While the job is in-progress, Output / OutputText are empty. After the
// job terminates (completed / failed / cancelled), the response is fully
// populated and identical to what a synchronous POST would return.
//
// @Summary Retrieve a Response by id
// @Description Polls the state of a background `/v1/responses` job. Submit a
// @Description response with `"background": true` to get back a 202 + `id`, then
// @Description GET `/v1/responses/{id}` until `status` is terminal
// @Description (`completed`, `failed`, `cancelled`).
// @Tags responses
// @Produce json
// @Param id path string true "Response id (resp_...)"
// @Success 200 {object} ResponsesObject "Current state of the response"
// @Failure 404 {object} APIError "Response id not found (never existed or evicted)"
// @Security BearerAuth
// @Router /v1/responses/{id} [get]
func NewResponseGetHandler(store *JobStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeAPIError(w, http.StatusMethodNotAllowed,
				"invalid_request_error", "method_not_allowed",
				"only GET is allowed")
			return
		}

		id := r.PathValue("id")
		if id == "" {
			writeAPIError(w, http.StatusBadRequest,
				"invalid_request_error", "missing_id",
				"response id is required")
			return
		}

		job, ok := store.Get(id)
		if !ok {
			writeAPIError(w, http.StatusNotFound,
				"invalid_request_error", "response_not_found",
				"no response found with id "+id)
			return
		}

		// Terminal: return the stored result verbatim.
		if job.Result != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(job.Result)
			return
		}

		// Non-terminal (in_progress / cancelled-without-result): emit a
		// minimal stub with the current Status.
		writeJobStub(w, id, job.Status)
	})
}

// writeJobStub emits a minimal ResponsesObject envelope for a non-terminal
// job state. Shared by GET and cancel handlers when there is no Result yet.
func writeJobStub(w http.ResponseWriter, id, status string) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ResponsesObject{
		ID:                id,
		Object:            "response",
		Status:            status,
		Output:            []ResponsesOutputItem{},
		OutputText:        "",
		Metadata:          map[string]string{},
		ParallelToolCalls: true,
		Store:             true,
		Temperature:       1.0,
		ToolChoice:        "auto",
		Tools:             []any{},
		TopP:              1.0,
		Truncation:        "disabled",
	})
}

// NewResponseCancelHandler returns an http.Handler for
// POST /v1/responses/{id}/cancel.
//
// Cancels an in-progress background job. The job's context is cancelled and
// status flips to "cancelled" immediately; the underlying goroutine may
// continue running but its result is discarded (Complete is a no-op once
// status == "cancelled").
//
// @Summary Cancel an in-progress background Response
// @Description Cancels a background `/v1/responses` job. Idempotent — calling
// @Description cancel on a job that has already completed or been cancelled
// @Description returns 200 with the current state.
// @Tags responses
// @Produce json
// @Param id path string true "Response id (resp_...)"
// @Success 200 {object} ResponsesObject "Job state after cancel"
// @Failure 404 {object} APIError "Response id not found"
// @Security BearerAuth
// @Router /v1/responses/{id}/cancel [post]
func NewResponseCancelHandler(store *JobStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeAPIError(w, http.StatusMethodNotAllowed,
				"invalid_request_error", "method_not_allowed",
				"only POST is allowed")
			return
		}

		id := r.PathValue("id")
		if id == "" {
			writeAPIError(w, http.StatusBadRequest,
				"invalid_request_error", "missing_id",
				"response id is required")
			return
		}

		job, ok := store.Cancel(id)
		if !ok {
			writeAPIError(w, http.StatusNotFound,
				"invalid_request_error", "response_not_found",
				"no response found with id "+id)
			return
		}

		if job.Result != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(job.Result)
			return
		}
		writeJobStub(w, id, job.Status)
	})
}
