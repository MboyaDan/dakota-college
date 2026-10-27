package mpesa

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// Daraja is the small part of Safaricom's API the college uses: an access
// token, registering the C2B callback URLs, and (sandbox only) simulating
// a payment. Hosts, paths and keys all come from configuration.
type Daraja struct {
	baseURL, key, secret    string
	registerPath, simulPath string
	client                  *http.Client
	now                     func() time.Time

	mu      sync.Mutex
	token   string
	expires time.Time
}

// ErrDaraja is a refusal or failure from Safaricom; its text is safe to show.
type ErrDaraja struct{ Message string }

func (e *ErrDaraja) Error() string { return "Safaricom: " + e.Message }

func (d *Daraja) accessToken(ctx context.Context) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.token != "" && d.now().Before(d.expires) {
		return d.token, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.baseURL+"/oauth/v1/generate?grant_type=client_credentials", nil)
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(d.key, d.secret)
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   string `json:"expires_in"`
	}
	if err := d.send(req, &out); err != nil {
		return "", err
	}
	if out.AccessToken == "" {
		return "", &ErrDaraja{Message: "no access token; check MPESA_CONSUMER_KEY and MPESA_CONSUMER_SECRET"}
	}
	secs, err := strconv.Atoi(out.ExpiresIn)
	if err != nil || secs <= 0 {
		secs = 3599
	}
	d.token = out.AccessToken
	// Renew a minute early so a token never expires on the way.
	d.expires = d.now().Add(time.Duration(secs)*time.Second - time.Minute)
	return d.token, nil
}

func (d *Daraja) post(ctx context.Context, path string, body, out any) error {
	token, err := d.accessToken(ctx)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.baseURL+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	return d.send(req, out)
}

func (d *Daraja) send(req *http.Request, out any) error {
	res, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("reach Safaricom: %w", err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return err
	}
	if res.StatusCode >= 300 {
		var e struct {
			ErrorMessage string `json:"errorMessage"`
			ResponseDesc string `json:"ResponseDescription"`
		}
		_ = json.Unmarshal(raw, &e)
		msg := e.ErrorMessage
		if msg == "" {
			msg = e.ResponseDesc
		}
		if msg == "" {
			msg = fmt.Sprintf("answered %d", res.StatusCode)
		}
		return &ErrDaraja{Message: msg}
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return &ErrDaraja{Message: "an answer that could not be read"}
	}
	return nil
}

type registerRequest struct {
	ShortCode       string `json:"ShortCode"`
	ResponseType    string `json:"ResponseType"`
	ConfirmationURL string `json:"ConfirmationURL"`
	ValidationURL   string `json:"ValidationURL"`
}

type darajaAnswer struct {
	ResponseCode        string `json:"ResponseCode"`
	ResponseDescription string `json:"ResponseDescription"`
	OriginatorID        string `json:"OriginatorCoversationID"`
}

func (d *Daraja) register(ctx context.Context, r registerRequest) (string, error) {
	var out darajaAnswer
	if err := d.post(ctx, d.registerPath, r, &out); err != nil {
		return "", err
	}
	if out.ResponseCode != "" && out.ResponseCode != "0" {
		return "", &ErrDaraja{Message: out.ResponseDescription}
	}
	return out.ResponseDescription, nil
}

type simulateRequest struct {
	ShortCode     string `json:"ShortCode"`
	CommandID     string `json:"CommandID"`
	Amount        string `json:"Amount"`
	Msisdn        string `json:"Msisdn"`
	BillRefNumber string `json:"BillRefNumber"`
}

func (d *Daraja) simulate(ctx context.Context, r simulateRequest) (string, error) {
	var out darajaAnswer
	if err := d.post(ctx, d.simulPath, r, &out); err != nil {
		return "", err
	}
	if out.ResponseCode != "" && out.ResponseCode != "0" {
		return "", &ErrDaraja{Message: out.ResponseDescription}
	}
	return out.ResponseDescription, nil
}

var errNoDaraja = errors.New("M-Pesa is not configured")
