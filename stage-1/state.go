package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = bcrypt.DefaultCost

type User struct {
	ID           string `json:"id"`
	Email        string `json:"email"`
	PasswordHash string `json:"password_hash"`
	DisplayName  string `json:"display_name"`
	Handle       string `json:"handle"`
	Balance      int64  `json:"balance"`
}

type Payment struct {
	ID           string  `json:"id"`
	FromID       string  `json:"from_id"`
	ToID         string  `json:"to_id"`
	Amount       int64   `json:"amount"`
	Note         string  `json:"note"`
	Visibility   string  `json:"visibility"`
	RequestID    *string `json:"request_id"`
	SettlementID *string `json:"settlement_id"`
	CreatedAt    string  `json:"created_at"`
	Seq          int64   `json:"seq"`
}

type MoneyRequest struct {
	ID          string  `json:"id"`
	RequesterID string  `json:"requester_id"`
	PayerID     string  `json:"payer_id"`
	Amount      int64   `json:"amount"`
	Note        string  `json:"note"`
	Status      string  `json:"status"`
	PaymentID   *string `json:"payment_id"`
	CreatedAt   string  `json:"created_at"`
	Seq         int64   `json:"seq"`
}

type IdemRec struct {
	UserID   string          `json:"user_id"`
	Method   string          `json:"method"`
	Path     string          `json:"path"`
	Key      string          `json:"key"`
	Canon    json.RawMessage `json:"canon"`
	Response json.RawMessage `json:"response"`
}

type world struct {
	Currency   string                   `json:"currency"`
	MinorUnits int                      `json:"minor_units"`
	Users      map[string]*User         `json:"users"`
	Payments   []*Payment               `json:"payments"`
	Requests   map[string]*MoneyRequest `json:"requests"`
	Tokens     map[string]string        `json:"tokens"`
	Operators  map[string]bool          `json:"operators"`
	Idem       []*IdemRec               `json:"idempotency"`
	Seq        int64                    `json:"seq"`
	LastStamp  string                   `json:"last_stamp"`

	emailIndex  map[string]string
	handleIndex map[string]string
	idemIndex   map[string]*IdemRec
}

func newWorld() *world {
	w := &world{
		Currency:   "EUR",
		MinorUnits: 2,
		Users:      map[string]*User{},
		Payments:   []*Payment{},
		Requests:   map[string]*MoneyRequest{},
		Tokens:     map[string]string{},
		Operators:  map[string]bool{},
		Idem:       []*IdemRec{},
	}
	w.prepare()
	return w
}

func (w *world) prepare() {
	if w.Users == nil {
		w.Users = map[string]*User{}
	}
	if w.Payments == nil {
		w.Payments = []*Payment{}
	}
	if w.Requests == nil {
		w.Requests = map[string]*MoneyRequest{}
	}
	if w.Tokens == nil {
		w.Tokens = map[string]string{}
	}
	if w.Operators == nil {
		w.Operators = map[string]bool{}
	}
	if w.Idem == nil {
		w.Idem = []*IdemRec{}
	}
	w.emailIndex = map[string]string{}
	w.handleIndex = map[string]string{}
	for id, u := range w.Users {
		if u == nil {
			continue
		}
		w.emailIndex[u.Email] = id
		w.handleIndex[u.Handle] = id
	}
	w.idemIndex = map[string]*IdemRec{}
	for _, rec := range w.Idem {
		if rec == nil {
			continue
		}
		w.idemIndex[idemComposite(rec.UserID, rec.Method, rec.Path, rec.Key)] = rec
	}
	var maxSeq int64
	maxStamp := w.LastStamp
	for _, p := range w.Payments {
		if p == nil {
			continue
		}
		if p.Seq > maxSeq {
			maxSeq = p.Seq
		}
		if p.CreatedAt > maxStamp {
			maxStamp = p.CreatedAt
		}
	}
	for _, rq := range w.Requests {
		if rq == nil {
			continue
		}
		if rq.Seq > maxSeq {
			maxSeq = rq.Seq
		}
		if rq.CreatedAt > maxStamp {
			maxStamp = rq.CreatedAt
		}
	}
	if w.Seq < maxSeq {
		w.Seq = maxSeq
	}
	w.LastStamp = maxStamp
}

func idemComposite(userID, method, path, key string) string {
	return userID + "\x00" + method + "\x00" + path + "\x00" + key
}

func (w *world) stamp() string {
	s := formatTime(time.Now())
	if w.LastStamp != "" && s < w.LastStamp {
		s = w.LastStamp
	}
	w.LastStamp = s
	return s
}

func formatTime(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05") + "+00:00"
}

func newID(prefix string) string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return prefix + hex.EncodeToString(b[:])
}

func newToken() string {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}

func strPtr(s string) *string {
	return &s
}

func passwordKey(pw string) []byte {
	b := []byte(pw)
	if len(b) > 72 {
		sum := sha256.Sum256(b)
		return sum[:]
	}
	return b
}

func hashPassword(pw string) (string, error) {
	h, err := bcrypt.GenerateFromPassword(passwordKey(pw), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

func checkPassword(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), passwordKey(pw)) == nil
}

func deriveHandle(email string) string {
	local := email
	if i := strings.IndexByte(email, '@'); i >= 0 {
		local = email[:i]
	}
	local = strings.ToLower(local)
	var b strings.Builder
	for _, r := range local {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	rs := []rune(b.String())
	if len(rs) > 20 {
		rs = rs[:20]
	}
	return string(rs)
}

func validEmail(s string) bool {
	local, domain, ok := strings.Cut(s, "@")
	if !ok || local == "" || domain == "" || strings.Contains(domain, "@") {
		return false
	}
	if strings.ContainsAny(local, " \t\r\n") || strings.ContainsAny(domain, " \t\r\n") {
		return false
	}
	return true
}

func equalSplit(amount int64, n int) []int64 {
	base := amount / int64(n)
	rem := amount % int64(n)
	out := make([]int64, n)
	for i := range out {
		out[i] = base
		if int64(i) < rem {
			out[i]++
		}
	}
	return out
}

func (w *world) nextSeq() int64 {
	w.Seq++
	return w.Seq
}

func (w *world) move(fromID, toID string, amount int64) *apiError {
	from := w.Users[fromID]
	to := w.Users[toID]
	if from == nil || to == nil {
		return &apiError{Status: 404, Code: "not_found", Msg: "no such user"}
	}
	if from.Balance < amount {
		return funds()
	}
	if amount > 0 && to.Balance > maxBalance-amount {
		return validation("balance would exceed the allowed range")
	}
	from.Balance -= amount
	to.Balance += amount
	return nil
}

func (w *world) addPayment(fromID, toID string, amount int64, note, vis string, requestID, settlementID *string, ts string) *Payment {
	p := &Payment{
		ID:           newID("p_"),
		FromID:       fromID,
		ToID:         toID,
		Amount:       amount,
		Note:         note,
		Visibility:   vis,
		RequestID:    requestID,
		SettlementID: settlementID,
		CreatedAt:    ts,
		Seq:          w.nextSeq(),
	}
	w.Payments = append(w.Payments, p)
	return p
}

func (w *world) addRequest(requesterID, payerID string, amount int64, note, ts string) *MoneyRequest {
	rq := &MoneyRequest{
		ID:          newID("rq_"),
		RequesterID: requesterID,
		PayerID:     payerID,
		Amount:      amount,
		Note:        note,
		Status:      "pending",
		CreatedAt:   ts,
		Seq:         w.nextSeq(),
	}
	w.Requests[rq.ID] = rq
	return rq
}

type PaymentView struct {
	PaymentID    string  `json:"payment_id"`
	FromUserID   string  `json:"from_user_id"`
	FromHandle   string  `json:"from_handle"`
	ToUserID     string  `json:"to_user_id"`
	ToHandle     string  `json:"to_handle"`
	Amount       int64   `json:"amount"`
	Currency     string  `json:"currency"`
	Note         string  `json:"note"`
	Visibility   string  `json:"visibility"`
	RequestID    *string `json:"request_id"`
	SettlementID *string `json:"settlement_id"`
	CreatedAt    string  `json:"created_at"`
}

type RequestView struct {
	RequestID       string  `json:"request_id"`
	RequesterID     string  `json:"requester_id"`
	RequesterHandle string  `json:"requester_handle"`
	PayerID         string  `json:"payer_id"`
	PayerHandle     string  `json:"payer_handle"`
	Amount          int64   `json:"amount"`
	Currency        string  `json:"currency"`
	Note            string  `json:"note"`
	Status          string  `json:"status"`
	PaymentID       *string `json:"payment_id"`
	CreatedAt       string  `json:"created_at"`
}

type ShareView struct {
	Handle string `json:"handle"`
	Amount int64  `json:"amount"`
}

type SplitView struct {
	SplitID   string        `json:"split_id"`
	Amount    int64         `json:"amount"`
	Currency  string        `json:"currency"`
	Note      string        `json:"note"`
	Shares    []ShareView   `json:"shares"`
	Requests  []RequestView `json:"requests"`
	CreatedAt string        `json:"created_at"`
}

type SettlementView struct {
	SettlementID string        `json:"settlement_id"`
	CommittedAt  string        `json:"committed_at"`
	Payments     []PaymentView `json:"payments"`
}

func (w *world) paymentView(p *Payment) PaymentView {
	v := PaymentView{
		PaymentID:    p.ID,
		FromUserID:   p.FromID,
		ToUserID:     p.ToID,
		Amount:       p.Amount,
		Currency:     w.Currency,
		Note:         p.Note,
		Visibility:   p.Visibility,
		RequestID:    p.RequestID,
		SettlementID: p.SettlementID,
		CreatedAt:    p.CreatedAt,
	}
	if u := w.Users[p.FromID]; u != nil {
		v.FromHandle = u.Handle
	}
	if u := w.Users[p.ToID]; u != nil {
		v.ToHandle = u.Handle
	}
	return v
}

func (w *world) requestView(rq *MoneyRequest) RequestView {
	v := RequestView{
		RequestID:   rq.ID,
		RequesterID: rq.RequesterID,
		PayerID:     rq.PayerID,
		Amount:      rq.Amount,
		Currency:    w.Currency,
		Note:        rq.Note,
		Status:      rq.Status,
		PaymentID:   rq.PaymentID,
		CreatedAt:   rq.CreatedAt,
	}
	if u := w.Users[rq.RequesterID]; u != nil {
		v.RequesterHandle = u.Handle
	}
	if u := w.Users[rq.PayerID]; u != nil {
		v.PayerHandle = u.Handle
	}
	return v
}

func validStatus(s string) bool {
	switch s {
	case "pending", "paid", "declined", "cancelled":
		return true
	default:
		return false
	}
}
