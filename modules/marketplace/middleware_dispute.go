package marketplace

import (
	"net/http"
	"strconv"

	"github.com/gocraft/web"
)

func (c *Context) DisputeMiddleware(w web.ResponseWriter, r *web.Request, next web.NextMiddlewareFunc) {

	dispute, err := FindDisputeByID(r.PathParams["dispute_uuid"])
	if err != nil {
		http.NotFound(w, r.Request)
		return
	}
	c.Dispute = *dispute

	transaction, err := FindTransactionByDisputeUuid(c.Dispute.Uuid)
	if err != nil {
		http.NotFound(w, r.Request)
		return
	}
	c.ViewTransaction = transaction.ViewTransaction()

	if (!c.ViewUser.IsAdmin && !c.ViewUser.IsStaff) && (transaction.BuyerUuid != c.ViewUser.Uuid && transaction.SellerUuid != c.ViewUser.Uuid) {
		http.NotFound(w, r.Request)
		return
	}

	if c.ViewUser.IsStaff && dispute.ResolverUserUuid != "" && dispute.ResolverUserUuid != c.ViewUser.Uuid {
		http.NotFound(w, r.Request)
		return
	}

	next(w, r)
}

func (c *Context) DisputeClaimMiddleware(w web.ResponseWriter, r *web.Request, next web.NextMiddlewareFunc) {

	claimID, err := strconv.ParseInt(r.PathParams["dispute_claim_id"], 10, 32)
	if err != nil || claimID < 0 {
		http.NotFound(w, r.Request)
		return
	}

	for _, claim := range c.Dispute.DisputeClaims {
		if claim.ID == uint(claimID) {
			c.DisputeClaim = claim
		}
	}

	if c.DisputeClaim.ID == uint(0) {
		http.NotFound(w, r.Request)
		return
	}

	thread, err := GetDisputeClaimThread(c.DisputeClaim)
	if err != nil {
		http.NotFound(w, r.Request)
		return
	}
	c.Thread = *thread
	c.ViewThread = thread.ViewThread(c.ViewUser.Language, c.ViewUser.User)

	next(w, r)
}

func (c *Context) DisputeStatsMiddleware(w web.ResponseWriter, r *web.Request, next web.NextMiddlewareFunc) {
	c.NumberOfDisputes = CountDisputesForUserUuid(c.ViewUser.Uuid)

	next(w, r)
}
