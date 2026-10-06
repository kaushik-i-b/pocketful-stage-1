package main

import (
	"encoding/json"
	"time"
)

func worldFromFixture(obj map[string]any) (*world, *apiError) {
	currency, ok := obj["currency"].(string)
	if !ok || currency == "" {
		return nil, validation("currency is required")
	}
	minorRaw, ok := obj["minor_units"]
	if !ok {
		return nil, validation("minor_units is required")
	}
	minor, ok := parseIntegral(minorRaw)
	if !ok || (minor != 0 && minor != 2 && minor != 3) {
		return nil, validation("minor_units must be 0, 2, or 3")
	}
	usersRaw, ok := obj["users"]
	if !ok {
		return nil, validation("users is required")
	}
	usersArr, ok := usersRaw.([]any)
	if !ok {
		return nil, validation("users must be an array")
	}
	payArr, aerr := optionalArray(obj, "payments")
	if aerr != nil {
		return nil, aerr
	}
	reqArr, aerr := optionalArray(obj, "requests")
	if aerr != nil {
		return nil, aerr
	}
	opArr, aerr := optionalArray(obj, "settlement_operator_ids")
	if aerr != nil {
		return nil, aerr
	}

	w := newWorld()
	w.Currency = currency
	w.MinorUnits = int(minor)
	w.Users = map[string]*User{}
	w.Operators = map[string]bool{}

	emails := map[string]bool{}
	handles := map[string]bool{}
	passwords := map[string]string{}
	for _, item := range usersArr {
		uobj, ok := item.(map[string]any)
		if !ok {
			return nil, validation("user must be an object")
		}
		id, aerr := fixtureString(uobj, "id")
		if aerr != nil {
			return nil, aerr
		}
		email, aerr := fixtureString(uobj, "email")
		if aerr != nil {
			return nil, aerr
		}
		password, aerr := fixtureString(uobj, "password")
		if aerr != nil {
			return nil, aerr
		}
		display, aerr := fixtureString(uobj, "display_name")
		if aerr != nil {
			return nil, aerr
		}
		handle, aerr := fixtureString(uobj, "handle")
		if aerr != nil {
			return nil, aerr
		}
		if id == "" || len(id) > 64 {
			return nil, validation("user id is invalid")
		}
		if !validEmail(email) {
			return nil, validation("user email is invalid")
		}
		if !handleRe.MatchString(handle) {
			return nil, validation("user handle is invalid")
		}
		balRaw, ok := uobj["balance"]
		if !ok {
			return nil, validation("balance is required")
		}
		bal, ok := parseIntegral(balRaw)
		if !ok || bal < 0 || bal > maxBalance {
			return nil, validation("balance must be a non-negative integer")
		}
		if _, exists := w.Users[id]; exists || emails[email] || handles[handle] {
			return nil, validation("duplicate user")
		}
		emails[email] = true
		handles[handle] = true
		passwords[id] = password
		w.Users[id] = &User{
			ID:          id,
			Email:       email,
			DisplayName: display,
			Handle:      handle,
			Balance:     bal,
		}
	}

	base := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	step := 0
	nextStamp := func() string {
		step++
		return formatTime(base.Add(time.Duration(step) * time.Second))
	}

	seenPay := map[string]bool{}
	for _, item := range payArr {
		pobj, ok := item.(map[string]any)
		if !ok {
			return nil, validation("payment must be an object")
		}
		id, aerr := fixtureString(pobj, "id")
		if aerr != nil {
			return nil, aerr
		}
		fromID, aerr := fixtureString(pobj, "from_user_id")
		if aerr != nil {
			return nil, aerr
		}
		toID, aerr := fixtureString(pobj, "to_user_id")
		if aerr != nil {
			return nil, aerr
		}
		if id == "" || len(id) > 64 || seenPay[id] {
			return nil, validation("payment id is invalid")
		}
		if w.Users[fromID] == nil || w.Users[toID] == nil {
			return nil, validation("payment user does not exist")
		}
		amtRaw, ok := pobj["amount"]
		if !ok {
			return nil, validation("payment amount is required")
		}
		amt, ok := parseIntegral(amtRaw)
		if !ok || amt < 0 || amt > maxAmount {
			return nil, validation("payment amount is invalid")
		}
		note := ""
		if v, ok := pobj["note"]; ok {
			s, ok := v.(string)
			if !ok {
				return nil, validation("payment note must be a string")
			}
			note = s
		}
		vis := "public"
		if v, ok := pobj["visibility"]; ok {
			s, ok := v.(string)
			if !ok || (s != "public" && s != "private") {
				return nil, validation("payment visibility is invalid")
			}
			vis = s
		}
		var requestID, settlementID *string
		if v, ok := pobj["request_id"]; ok && v != nil {
			s, ok := v.(string)
			if !ok {
				return nil, validation("payment request_id is invalid")
			}
			requestID = strPtr(s)
		}
		if v, ok := pobj["settlement_id"]; ok && v != nil {
			s, ok := v.(string)
			if !ok {
				return nil, validation("payment settlement_id is invalid")
			}
			settlementID = strPtr(s)
		}
		seenPay[id] = true
		ts := nextStamp()
		w.Payments = append(w.Payments, &Payment{
			ID:           id,
			FromID:       fromID,
			ToID:         toID,
			Amount:       amt,
			Note:         note,
			Visibility:   vis,
			RequestID:    requestID,
			SettlementID: settlementID,
			CreatedAt:    ts,
			Seq:          w.nextSeq(),
		})
	}

	for _, item := range reqArr {
		robj, ok := item.(map[string]any)
		if !ok {
			return nil, validation("request must be an object")
		}
		id, aerr := fixtureString(robj, "id")
		if aerr != nil {
			return nil, aerr
		}
		requesterID, aerr := fixtureString(robj, "requester_id")
		if aerr != nil {
			return nil, aerr
		}
		payerID, aerr := fixtureString(robj, "payer_id")
		if aerr != nil {
			return nil, aerr
		}
		if id == "" || len(id) > 64 {
			return nil, validation("request id is invalid")
		}
		if _, exists := w.Requests[id]; exists {
			return nil, validation("duplicate request id")
		}
		if w.Users[requesterID] == nil || w.Users[payerID] == nil {
			return nil, validation("request user does not exist")
		}
		amtRaw, ok := robj["amount"]
		if !ok {
			return nil, validation("request amount is required")
		}
		amt, ok := parseIntegral(amtRaw)
		if !ok || amt < 0 || amt > maxAmount {
			return nil, validation("request amount is invalid")
		}
		note := ""
		if v, ok := robj["note"]; ok {
			s, ok := v.(string)
			if !ok {
				return nil, validation("request note must be a string")
			}
			note = s
		}
		status, aerr := fixtureString(robj, "status")
		if aerr != nil {
			return nil, aerr
		}
		if !validStatus(status) {
			return nil, validation("request status is invalid")
		}
		var paymentID *string
		if v, ok := robj["payment_id"]; ok && v != nil {
			s, ok := v.(string)
			if !ok || !seenPay[s] {
				return nil, validation("request payment_id is invalid")
			}
			paymentID = strPtr(s)
		}
		ts := nextStamp()
		w.Requests[id] = &MoneyRequest{
			ID:          id,
			RequesterID: requesterID,
			PayerID:     payerID,
			Amount:      amt,
			Note:        note,
			Status:      status,
			PaymentID:   paymentID,
			CreatedAt:   ts,
			Seq:         w.nextSeq(),
		}
	}

	for _, item := range opArr {
		id, ok := item.(string)
		if !ok || w.Users[id] == nil {
			return nil, validation("settlement operator does not exist")
		}
		w.Operators[id] = true
	}

	for id, pw := range passwords {
		hash, err := hashPassword(pw)
		if err != nil {
			return nil, validation("could not store password")
		}
		w.Users[id].PasswordHash = hash
	}
	w.prepare()
	return w, nil
}

func optionalArray(obj map[string]any, field string) ([]any, *apiError) {
	v, ok := obj[field]
	if !ok {
		return []any{}, nil
	}
	arr, ok := v.([]any)
	if !ok {
		return nil, validation(field + " must be an array")
	}
	return arr, nil
}

func fixtureString(obj map[string]any, field string) (string, *apiError) {
	v, ok := obj[field]
	if !ok {
		return "", validation(field + " is required")
	}
	s, ok := v.(string)
	if !ok {
		return "", validation(field + " must be a string")
	}
	return s, nil
}

func validateImported(w *world) *apiError {
	if w == nil || w.Currency == "" || (w.MinorUnits != 0 && w.MinorUnits != 2 && w.MinorUnits != 3) {
		return validation("invalid currency state")
	}
	if w.Users == nil {
		return validation("invalid user state")
	}
	emails := map[string]bool{}
	handles := map[string]bool{}
	for id, u := range w.Users {
		if u == nil || u.ID != id || u.ID == "" || len(u.ID) > 64 {
			return validation("invalid user")
		}
		if u.PasswordHash == "" || !validEmail(u.Email) || !handleRe.MatchString(u.Handle) {
			return validation("invalid user")
		}
		if u.Balance < 0 || u.Balance > maxBalance {
			return validation("invalid balance")
		}
		if emails[u.Email] || handles[u.Handle] {
			return validation("duplicate user")
		}
		emails[u.Email] = true
		handles[u.Handle] = true
	}
	seenPay := map[string]bool{}
	for _, p := range w.Payments {
		if p == nil || p.ID == "" || len(p.ID) > 64 || seenPay[p.ID] {
			return validation("invalid payment")
		}
		if w.Users[p.FromID] == nil || w.Users[p.ToID] == nil {
			return validation("invalid payment")
		}
		if p.Amount < 0 || p.Amount > maxAmount {
			return validation("invalid payment")
		}
		if p.Visibility != "public" && p.Visibility != "private" {
			return validation("invalid payment")
		}
		if p.CreatedAt == "" {
			return validation("invalid payment")
		}
		seenPay[p.ID] = true
	}
	for id, rq := range w.Requests {
		if rq == nil || rq.ID != id || rq.ID == "" {
			return validation("invalid request")
		}
		if w.Users[rq.RequesterID] == nil || w.Users[rq.PayerID] == nil {
			return validation("invalid request")
		}
		if !validStatus(rq.Status) || rq.Amount < 0 || rq.Amount > maxAmount || rq.CreatedAt == "" {
			return validation("invalid request")
		}
		if rq.PaymentID != nil && !seenPay[*rq.PaymentID] {
			return validation("invalid request")
		}
	}
	for tok, uid := range w.Tokens {
		if tok == "" || w.Users[uid] == nil {
			return validation("invalid token")
		}
	}
	for uid := range w.Operators {
		if w.Users[uid] == nil {
			return validation("invalid operator")
		}
	}
	for _, rec := range w.Idem {
		if rec == nil || rec.UserID == "" || rec.Method == "" || rec.Path == "" || rec.Key == "" {
			return validation("invalid idempotency state")
		}
		if w.Users[rec.UserID] == nil || !json.Valid(rec.Canon) || !json.Valid(rec.Response) {
			return validation("invalid idempotency state")
		}
		canon, err := recanon(rec.Canon)
		if err != nil {
			return validation("invalid idempotency state")
		}
		rec.Canon = canon
	}
	w.prepare()
	return nil
}

func worldFromImport(obj map[string]any) (*world, *apiError) {
	track, _ := obj["track"].(string)
	if track != "pocketful" {
		return nil, validation("track must be pocketful")
	}
	ver, ok := parseIntegral(obj["format_version"])
	if !ok || ver != 1 {
		return nil, validation("format_version must be 1")
	}
	state, ok := obj["state"]
	if !ok || state == nil {
		return nil, validation("state is required")
	}
	if _, isObj := state.(map[string]any); !isObj {
		return nil, validation("state must be an object")
	}
	raw, err := json.Marshal(norm(state))
	if err != nil {
		return nil, validation("state is invalid")
	}
	w := &world{}
	if err := json.Unmarshal(raw, w); err != nil {
		return nil, validation("state is invalid")
	}
	if aerr := validateImported(w); aerr != nil {
		return nil, aerr
	}
	return w, nil
}
