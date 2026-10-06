package main

import (
	"bytes"
	"net/http"
	"sort"
	"strings"
	"unicode/utf8"
)

func (a *App) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *App) reset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusNotFound, "not_found", "not found")
		return
	}
	body, err := readBody(r)
	if err != nil {
		malformed("unreadable body").write(w)
		return
	}
	obj, err := parseObject(body, false)
	if err != nil {
		malformed("request body must be a JSON object").write(w)
		return
	}
	nw, aerr := worldFromFixture(obj)
	if aerr != nil {
		aerr.write(w)
		return
	}
	a.mu.Lock()
	a.world = nw
	a.mu.Unlock()
	writeEmpty(w)
}

func (a *App) export(w http.ResponseWriter, _ *http.Request) {
	a.mu.Lock()
	body, err := marshalJSON(a.world)
	a.mu.Unlock()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "export failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"track":          "pocketful",
		"format_version": 1,
		"state":          jsonRaw(body),
	})
}

func (a *App) importState(w http.ResponseWriter, r *http.Request) {
	body, err := readBody(r)
	if err != nil {
		malformed("unreadable body").write(w)
		return
	}
	obj, err := parseObject(body, false)
	if err != nil {
		malformed("request body must be a JSON object").write(w)
		return
	}
	nw, aerr := worldFromImport(obj)
	if aerr != nil {
		aerr.write(w)
		return
	}
	a.mu.Lock()
	a.world = nw
	a.mu.Unlock()
	writeEmpty(w)
}

func (a *App) signup(w http.ResponseWriter, r *http.Request) {
	obj, ok := readObj(w, r, false)
	if !ok {
		return
	}
	email, aerr := requireString(obj, "email")
	if aerr != nil {
		aerr.write(w)
		return
	}
	password, aerr := requireString(obj, "password")
	if aerr != nil {
		aerr.write(w)
		return
	}
	display, aerr := requireString(obj, "display_name")
	if aerr != nil {
		aerr.write(w)
		return
	}
	if !validEmail(email) {
		validation("email must be of the form local@domain").write(w)
		return
	}
	if utf8.RuneCountInString(password) < 8 {
		validation("password must be at least 8 characters").write(w)
		return
	}
	handle := deriveHandle(email)
	if !handleRe.MatchString(handle) {
		validation("could not derive a handle from the email").write(w)
		return
	}
	hash, err := hashPassword(password)
	if err != nil {
		validation("could not store password").write(w)
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, taken := a.world.emailIndex[email]; taken {
		writeErr(w, http.StatusConflict, "email_taken", "email is already registered")
		return
	}
	if _, taken := a.world.handleIndex[handle]; taken {
		writeErr(w, http.StatusConflict, "handle_taken", "handle is already taken")
		return
	}
	id := newID("u_")
	user := &User{
		ID:           id,
		Email:        email,
		PasswordHash: hash,
		DisplayName:  display,
		Handle:       handle,
		Balance:      0,
	}
	a.world.Users[id] = user
	a.world.emailIndex[email] = id
	a.world.handleIndex[handle] = id
	tok := newToken()
	a.world.Tokens[tok] = id
	writeJSON(w, http.StatusCreated, map[string]string{
		"user_id":      id,
		"display_name": display,
		"token":        tok,
	})
}

func (a *App) login(w http.ResponseWriter, r *http.Request) {
	obj, ok := readObj(w, r, false)
	if !ok {
		return
	}
	email, aerr := requireString(obj, "email")
	if aerr != nil {
		aerr.write(w)
		return
	}
	password, aerr := requireString(obj, "password")
	if aerr != nil {
		aerr.write(w)
		return
	}
	a.mu.Lock()
	id, found := a.world.emailIndex[email]
	user := a.world.Users[id]
	hash := ""
	if found && user != nil {
		hash = user.PasswordHash
	}
	a.mu.Unlock()
	if hash == "" || !checkPassword(hash, password) {
		writeErr(w, http.StatusUnauthorized, "unauthenticated", "unknown email or wrong password")
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	id, found = a.world.emailIndex[email]
	user = a.world.Users[id]
	if !found || user == nil || user.PasswordHash != hash {
		writeErr(w, http.StatusUnauthorized, "unauthenticated", "unknown email or wrong password")
		return
	}
	tok := newToken()
	a.world.Tokens[tok] = user.ID
	writeJSON(w, http.StatusOK, map[string]string{
		"user_id":      user.ID,
		"display_name": user.DisplayName,
		"token":        tok,
	})
}

func (a *App) me(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	user, aerr := a.auth(r)
	if aerr != nil {
		aerr.write(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"user_id":      user.ID,
		"display_name": user.DisplayName,
		"handle":       user.Handle,
		"balance":      user.Balance,
		"currency":     a.world.Currency,
		"minor_units":  a.world.MinorUnits,
	})
}

type idemCall struct {
	user  *User
	key   string
	canon []byte
	obj   map[string]any
	path  string
}

func (a *App) beginWrite(w http.ResponseWriter, r *http.Request, emptyOK, operator bool) (*idemCall, bool) {
	obj, ok := readObj(w, r, emptyOK)
	if !ok {
		return nil, false
	}
	canon, err := canonical(obj)
	if err != nil {
		malformed("request body must be a JSON object").write(w)
		return nil, false
	}
	a.mu.Lock()
	user, aerr := a.auth(r)
	if aerr != nil {
		a.mu.Unlock()
		aerr.write(w)
		return nil, false
	}
	key, kerr := readIdemKey(r)
	if kerr != nil {
		a.mu.Unlock()
		kerr.write(w)
		return nil, false
	}
	if a.replay(w, user.ID, r.Method, r.URL.Path, key, canon) {
		a.mu.Unlock()
		return nil, false
	}
	if operator && !a.world.Operators[user.ID] {
		a.mu.Unlock()
		writeErr(w, http.StatusForbidden, "forbidden", "operator permission is required")
		return nil, false
	}
	return &idemCall{user: user, key: key, canon: canon, obj: obj, path: r.URL.Path}, true
}

func (a *App) replay(w http.ResponseWriter, userID, method, path, key string, canon []byte) bool {
	rec := a.world.idemIndex[idemComposite(userID, method, path, key)]
	if rec == nil {
		return false
	}
	if bytes.Equal(rec.Canon, canon) {
		writeRaw(w, http.StatusOK, rec.Response)
		return true
	}
	writeErr(w, http.StatusConflict, "idempotency_key_reuse", "idempotency key was used with a different request")
	return true
}

func (a *App) commit(call *idemCall, resp []byte) {
	rec := &IdemRec{
		UserID:   call.user.ID,
		Method:   http.MethodPost,
		Path:     call.path,
		Key:      call.key,
		Canon:    append([]byte(nil), call.canon...),
		Response: append([]byte(nil), resp...),
	}
	a.world.Idem = append(a.world.Idem, rec)
	a.world.idemIndex[idemComposite(rec.UserID, rec.Method, rec.Path, rec.Key)] = rec
}

func (a *App) createPayment(w http.ResponseWriter, r *http.Request) {
	call, ok := a.beginWrite(w, r, false, false)
	if !ok {
		return
	}
	defer a.mu.Unlock()
	amount, aerr := requireAmount(call.obj)
	if aerr != nil {
		aerr.write(w)
		return
	}
	note, aerr := optionalNote(call.obj)
	if aerr != nil {
		aerr.write(w)
		return
	}
	vis, aerr := optionalVisibility(call.obj)
	if aerr != nil {
		aerr.write(w)
		return
	}
	handle, aerr := requireString(call.obj, "to_handle")
	if aerr != nil {
		aerr.write(w)
		return
	}
	to, aerr := a.resolveHandle(handle, call.user, "self_payment", "cannot pay yourself")
	if aerr != nil {
		aerr.write(w)
		return
	}
	if aerr = a.world.move(call.user.ID, to.ID, amount); aerr != nil {
		aerr.write(w)
		return
	}
	p := a.world.addPayment(call.user.ID, to.ID, amount, note, vis, nil, nil, a.world.stamp())
	body, err := marshalJSON(a.world.paymentView(p))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "encode failed")
		return
	}
	a.commit(call, body)
	writeRaw(w, http.StatusCreated, body)
}

func (a *App) createRequest(w http.ResponseWriter, r *http.Request) {
	call, ok := a.beginWrite(w, r, false, false)
	if !ok {
		return
	}
	defer a.mu.Unlock()
	amount, aerr := requireAmount(call.obj)
	if aerr != nil {
		aerr.write(w)
		return
	}
	note, aerr := optionalNote(call.obj)
	if aerr != nil {
		aerr.write(w)
		return
	}
	handle, aerr := requireString(call.obj, "payer_handle")
	if aerr != nil {
		aerr.write(w)
		return
	}
	payer, aerr := a.resolveHandle(handle, call.user, "self_request", "cannot request money from yourself")
	if aerr != nil {
		aerr.write(w)
		return
	}
	rq := a.world.addRequest(call.user.ID, payer.ID, amount, note, a.world.stamp())
	body, err := marshalJSON(a.world.requestView(rq))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "encode failed")
		return
	}
	a.commit(call, body)
	writeRaw(w, http.StatusCreated, body)
}

func (a *App) payRequest(w http.ResponseWriter, r *http.Request) {
	call, ok := a.beginWrite(w, r, true, false)
	if !ok {
		return
	}
	defer a.mu.Unlock()
	vis, aerr := optionalVisibility(call.obj)
	if aerr != nil {
		aerr.write(w)
		return
	}
	id := r.PathValue("id")
	rq := a.world.Requests[id]
	if rq == nil {
		writeErr(w, http.StatusNotFound, "not_found", "request not found")
		return
	}
	if rq.PayerID != call.user.ID {
		writeErr(w, http.StatusForbidden, "forbidden", "only the payer can pay this request")
		return
	}
	if rq.Status != "pending" {
		writeErr(w, http.StatusConflict, "request_not_pending", "request is not pending")
		return
	}
	if aerr = a.world.move(rq.PayerID, rq.RequesterID, rq.Amount); aerr != nil {
		aerr.write(w)
		return
	}
	ts := a.world.stamp()
	p := a.world.addPayment(rq.PayerID, rq.RequesterID, rq.Amount, rq.Note, vis, strPtr(rq.ID), nil, ts)
	rq.Status = "paid"
	rq.PaymentID = strPtr(p.ID)
	body, err := marshalJSON(a.world.paymentView(p))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "encode failed")
		return
	}
	a.commit(call, body)
	writeRaw(w, http.StatusCreated, body)
}

func (a *App) declineRequest(w http.ResponseWriter, r *http.Request) {
	_, _ = readBody(r)
	a.mu.Lock()
	defer a.mu.Unlock()
	user, aerr := a.auth(r)
	if aerr != nil {
		aerr.write(w)
		return
	}
	rq := a.world.Requests[r.PathValue("id")]
	if rq == nil {
		writeErr(w, http.StatusNotFound, "not_found", "request not found")
		return
	}
	if rq.PayerID != user.ID {
		writeErr(w, http.StatusForbidden, "forbidden", "only the payer can decline this request")
		return
	}
	if rq.Status == "declined" {
		writeJSON(w, http.StatusOK, a.world.requestView(rq))
		return
	}
	if rq.Status != "pending" {
		writeErr(w, http.StatusConflict, "request_not_pending", "request is not pending")
		return
	}
	rq.Status = "declined"
	writeJSON(w, http.StatusOK, a.world.requestView(rq))
}

func (a *App) cancelRequest(w http.ResponseWriter, r *http.Request) {
	_, _ = readBody(r)
	a.mu.Lock()
	defer a.mu.Unlock()
	user, aerr := a.auth(r)
	if aerr != nil {
		aerr.write(w)
		return
	}
	rq := a.world.Requests[r.PathValue("id")]
	if rq == nil {
		writeErr(w, http.StatusNotFound, "not_found", "request not found")
		return
	}
	if rq.RequesterID != user.ID {
		writeErr(w, http.StatusForbidden, "forbidden", "only the requester can cancel this request")
		return
	}
	if rq.Status == "cancelled" {
		writeJSON(w, http.StatusOK, a.world.requestView(rq))
		return
	}
	if rq.Status != "pending" {
		writeErr(w, http.StatusConflict, "request_not_pending", "request is not pending")
		return
	}
	rq.Status = "cancelled"
	writeJSON(w, http.StatusOK, a.world.requestView(rq))
}

func (a *App) listRequests(w http.ResponseWriter, r *http.Request) {
	limit, offset, huge, pageErr := parsePage(r)
	q := r.URL.Query()
	direction := ""
	var filterErr *apiError
	if vals, ok := q["direction"]; ok {
		if len(vals) == 0 || (vals[0] != "incoming" && vals[0] != "outgoing") {
			filterErr = validation("direction must be incoming or outgoing")
		} else {
			direction = vals[0]
		}
	}
	status := ""
	if filterErr == nil {
		if vals, ok := q["status"]; ok {
			if len(vals) == 0 || !validStatus(vals[0]) {
				filterErr = validation("status is invalid")
			} else {
				status = vals[0]
			}
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	user, aerr := a.auth(r)
	if aerr != nil {
		aerr.write(w)
		return
	}
	if pageErr != nil {
		pageErr.write(w)
		return
	}
	if filterErr != nil {
		filterErr.write(w)
		return
	}
	list := make([]*MoneyRequest, 0)
	for _, rq := range a.world.Requests {
		if rq == nil {
			continue
		}
		switch direction {
		case "incoming":
			if rq.PayerID != user.ID {
				continue
			}
		case "outgoing":
			if rq.RequesterID != user.ID {
				continue
			}
		default:
			if rq.PayerID != user.ID && rq.RequesterID != user.ID {
				continue
			}
		}
		if status != "" && rq.Status != status {
			continue
		}
		list = append(list, rq)
	}
	sortRequests(list)
	page, more := pageSlice(list, offset, limit, huge)
	views := make([]RequestView, 0, len(page))
	for _, rq := range page {
		views = append(views, a.world.requestView(rq))
	}
	writeJSON(w, http.StatusOK, map[string]any{"requests": views, "has_more": more})
}

func (a *App) activity(w http.ResponseWriter, r *http.Request) {
	limit, offset, huge, pageErr := parsePage(r)
	a.mu.Lock()
	defer a.mu.Unlock()
	user, aerr := a.auth(r)
	if aerr != nil {
		aerr.write(w)
		return
	}
	if pageErr != nil {
		pageErr.write(w)
		return
	}
	list := make([]*Payment, 0)
	for _, p := range a.world.Payments {
		if p == nil {
			continue
		}
		if p.Visibility == "public" || p.FromID == user.ID || p.ToID == user.ID {
			list = append(list, p)
		}
	}
	sortPayments(list)
	page, more := pageSlice(list, offset, limit, huge)
	views := make([]PaymentView, 0, len(page))
	for _, p := range page {
		views = append(views, a.world.paymentView(p))
	}
	writeJSON(w, http.StatusOK, map[string]any{"payments": views, "has_more": more})
}

func (a *App) createSplit(w http.ResponseWriter, r *http.Request) {
	call, ok := a.beginWrite(w, r, false, false)
	if !ok {
		return
	}
	defer a.mu.Unlock()
	amount, aerr := requireAmount(call.obj)
	if aerr != nil {
		aerr.write(w)
		return
	}
	note, aerr := optionalNote(call.obj)
	if aerr != nil {
		aerr.write(w)
		return
	}
	raw, ok := call.obj["participant_handles"]
	if !ok {
		validation("participant_handles is required").write(w)
		return
	}
	arr, ok := raw.([]any)
	if !ok {
		malformed("participant_handles must be an array").write(w)
		return
	}
	handles := make([]string, 0, len(arr))
	seen := map[string]bool{}
	for _, item := range arr {
		h, ok := item.(string)
		if !ok {
			malformed("participant handle must be a string").write(w)
			return
		}
		handles = append(handles, h)
	}
	if len(handles) == 0 {
		validation("participant_handles must not be empty").write(w)
		return
	}
	for _, h := range handles {
		if seen[h] {
			validation("participant_handles contains a duplicate").write(w)
			return
		}
		seen[h] = true
	}
	participants := make([]*User, 0, len(handles))
	for _, h := range handles {
		id, found := a.world.handleIndex[h]
		u := a.world.Users[id]
		if !found || u == nil {
			writeErr(w, http.StatusNotFound, "not_found", "no such handle")
			return
		}
		participants = append(participants, u)
	}
	sharesAmt := equalSplit(amount, len(participants))
	shares := make([]ShareView, 0, len(participants))
	reqs := make([]RequestView, 0)
	ts := a.world.stamp()
	for i, u := range participants {
		shares = append(shares, ShareView{Handle: u.Handle, Amount: sharesAmt[i]})
		if u.ID == call.user.ID {
			continue
		}
		rq := a.world.addRequest(call.user.ID, u.ID, sharesAmt[i], note, ts)
		reqs = append(reqs, a.world.requestView(rq))
	}
	body, err := marshalJSON(SplitView{
		SplitID:   newID("sp_"),
		Amount:    amount,
		Currency:  a.world.Currency,
		Note:      note,
		Shares:    shares,
		Requests:  reqs,
		CreatedAt: ts,
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "encode failed")
		return
	}
	a.commit(call, body)
	writeRaw(w, http.StatusCreated, body)
}

type parsedTransfer struct {
	from, to *User
	amount   int64
	note     string
	vis      string
}

func (a *App) createSettlement(w http.ResponseWriter, r *http.Request) {
	call, ok := a.beginWrite(w, r, false, true)
	if !ok {
		return
	}
	defer a.mu.Unlock()
	raw, exists := call.obj["transfers"]
	if !exists {
		validation("transfers is required").write(w)
		return
	}
	arr, ok := raw.([]any)
	if !ok {
		validation("transfers must be an array").write(w)
		return
	}
	if len(arr) < 1 || len(arr) > 32 {
		validation("transfers must contain 1 to 32 entries").write(w)
		return
	}
	parsed := make([]parsedTransfer, 0, len(arr))
	for _, item := range arr {
		obj, ok := item.(map[string]any)
		if !ok {
			validation("transfer must be an object").write(w)
			return
		}
		amount, aerr := requireAmount(obj)
		if aerr != nil {
			aerr.write(w)
			return
		}
		note, aerr := optionalNote(obj)
		if aerr != nil {
			aerr.write(w)
			return
		}
		vis, aerr := optionalVisibility(obj)
		if aerr != nil {
			aerr.write(w)
			return
		}
		fromHandle, aerr := requireString(obj, "from_handle")
		if aerr != nil {
			if aerr.Code == "malformed_request" {
				validation("transfer handle must be a string").write(w)
				return
			}
			aerr.write(w)
			return
		}
		toHandle, aerr := requireString(obj, "to_handle")
		if aerr != nil {
			if aerr.Code == "malformed_request" {
				validation("transfer handle must be a string").write(w)
				return
			}
			aerr.write(w)
			return
		}
		fromID, fromOK := a.world.handleIndex[fromHandle]
		toID, toOK := a.world.handleIndex[toHandle]
		from := a.world.Users[fromID]
		to := a.world.Users[toID]
		if !fromOK || !toOK || from == nil || to == nil {
			writeErr(w, http.StatusNotFound, "not_found", "no such handle")
			return
		}
		if from.ID == to.ID {
			writeErr(w, http.StatusUnprocessableEntity, "self_payment", "cannot transfer to the same wallet")
			return
		}
		parsed = append(parsed, parsedTransfer{from: from, to: to, amount: amount, note: note, vis: vis})
	}
	delta := map[string]int64{}
	for _, tr := range parsed {
		delta[tr.from.ID] -= tr.amount
		delta[tr.to.ID] += tr.amount
	}
	for id, d := range delta {
		next := a.world.Users[id].Balance + d
		if next < 0 {
			funds().write(w)
			return
		}
		if next > maxBalance {
			validation("balance would exceed the allowed range").write(w)
			return
		}
	}
	for id, d := range delta {
		a.world.Users[id].Balance += d
	}
	ts := a.world.stamp()
	sid := newID("st_")
	views := make([]PaymentView, 0, len(parsed))
	for _, tr := range parsed {
		p := a.world.addPayment(tr.from.ID, tr.to.ID, tr.amount, tr.note, tr.vis, nil, strPtr(sid), ts)
		views = append(views, a.world.paymentView(p))
	}
	body, err := marshalJSON(SettlementView{SettlementID: sid, CommittedAt: ts, Payments: views})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "encode failed")
		return
	}
	a.commit(call, body)
	writeRaw(w, http.StatusCreated, body)
}

func (a *App) resolveHandle(handle string, caller *User, selfCode, selfMsg string) (*User, *apiError) {
	if caller != nil && handle == caller.Handle {
		return nil, &apiError{Status: http.StatusUnprocessableEntity, Code: selfCode, Msg: selfMsg}
	}
	id, ok := a.world.handleIndex[handle]
	u := a.world.Users[id]
	if !ok || u == nil {
		return nil, &apiError{Status: http.StatusNotFound, Code: "not_found", Msg: "no such handle"}
	}
	return u, nil
}

func (a *App) auth(r *http.Request) (*User, *apiError) {
	tok, ok := bearer(r)
	if !ok {
		return nil, &apiError{Status: http.StatusUnauthorized, Code: "unauthenticated", Msg: "authentication required"}
	}
	id, ok := a.world.Tokens[tok]
	u := a.world.Users[id]
	if !ok || u == nil {
		return nil, &apiError{Status: http.StatusUnauthorized, Code: "unauthenticated", Msg: "authentication required"}
	}
	return u, nil
}

func bearer(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	if h == "" {
		return "", false
	}
	scheme, token, ok := strings.Cut(h, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	token = strings.TrimSpace(token)
	if token == "" || strings.Contains(token, " ") {
		return "", false
	}
	return token, true
}

func readIdemKey(r *http.Request) (string, *apiError) {
	vals := r.Header.Values("Idempotency-Key")
	if len(vals) == 0 || vals[0] == "" {
		return "", &apiError{Status: http.StatusBadRequest, Code: "missing_idempotency_key", Msg: "idempotency key is required"}
	}
	key := vals[0]
	if utf8.RuneCountInString(key) > 255 {
		return "", validation("idempotency key must be 1 to 255 characters")
	}
	return key, nil
}

func readObj(w http.ResponseWriter, r *http.Request, emptyOK bool) (map[string]any, bool) {
	body, err := readBody(r)
	if err != nil {
		malformed("unreadable body").write(w)
		return nil, false
	}
	obj, err := parseObject(body, emptyOK)
	if err != nil {
		malformed("request body must be a JSON object").write(w)
		return nil, false
	}
	return obj, true
}

func sortPayments(list []*Payment) {
	sort.Slice(list, func(i, j int) bool {
		if list[i].CreatedAt != list[j].CreatedAt {
			return list[i].CreatedAt > list[j].CreatedAt
		}
		return list[i].Seq > list[j].Seq
	})
}

func sortRequests(list []*MoneyRequest) {
	sort.Slice(list, func(i, j int) bool {
		if list[i].CreatedAt != list[j].CreatedAt {
			return list[i].CreatedAt > list[j].CreatedAt
		}
		return list[i].Seq > list[j].Seq
	})
}

func jsonRaw(b []byte) jsonRawMessage {
	return jsonRawMessage(b)
}

type jsonRawMessage []byte

func (m jsonRawMessage) MarshalJSON() ([]byte, error) {
	if len(m) == 0 {
		return []byte("null"), nil
	}
	return m, nil
}
