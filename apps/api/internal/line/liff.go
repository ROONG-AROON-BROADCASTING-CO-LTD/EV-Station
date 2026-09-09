package line

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/rbc/ev-station/apps/api/internal/domain"
	"io"
	"net/http"
	"net/url"
	"strings"
)

var ErrIdentity = errors.New("invalid LINE identity")

// VerifyIDToken obtains identity only from LINE's verification response.
func (n *Notifier) VerifyIDToken(ctx context.Context, token, channelID string) (string, error) {
	if token == "" || channelID == "" {
		return "", ErrIdentity
	}
	form := url.Values{"id_token": {token}, "client_id": {channelID}}
	req, err := http.NewRequestWithContext(ctx, "POST", n.apiBaseURL+"/oauth2/v2.1/verify", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := n.client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	var identity struct {
		Sub string `json:"sub"`
		Aud string `json:"aud"`
	}
	if res.StatusCode != 200 || json.NewDecoder(io.LimitReader(res.Body, 16384)).Decode(&identity) != nil || identity.Aud != channelID || !strings.HasPrefix(identity.Sub, "U") {
		return "", ErrIdentity
	}
	return identity.Sub, nil
}

func (n *Notifier) ConfirmSubmission(ctx context.Context, user string, s domain.Site) error {
	if n.channelAccessToken == "" {
		return errors.New("LINE messaging unavailable")
	}
	referenceCode := s.ReferenceCode
	if referenceCode == "" {
		referenceCode = s.ID.String()
	}
	lines := []string{
		"ได้รับข้อมูลพื้นที่แล้วครับ",
		"เลขที่รายการ: " + referenceCode,
		"พื้นที่: " + s.Name,
		"ผู้ติดต่อ: " + s.ContactName,
		"โทร: " + s.ContactPhone,
		fmt.Sprintf("ขนาด: %g %s", s.LandSize, landSizeUnit(s.LandSizeUnit)),
	}
	if s.InternetAvailable != nil {
		internet := "ไม่มี"
		if *s.InternetAvailable {
			internet = "มี"
		}
		lines = append(lines, "สัญญาณอินเทอร์เน็ต: "+internet)
	}
	if s.FrontageMeters != nil {
		lines = append(lines, fmt.Sprintf("หน้ากว้างทางเข้า: %g เมตร", *s.FrontageMeters))
	}
	if s.GoogleMapsURL != "" {
		lines = append(lines, "Google Maps: "+s.GoogleMapsURL)
	}
	lines = append(lines, "ทีมงานจะตรวจสอบข้อมูลและติดต่อกลับในแชตนี้ครับ")
	text := strings.Join(lines, "\n")
	body, _ := json.Marshal(map[string]any{"to": user, "messages": []map[string]string{{"type": "text", "text": text}}})
	req, err := http.NewRequestWithContext(ctx, "POST", n.apiBaseURL+"/v2/bot/message/push", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+n.channelAccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Line-Retry-Key", s.ID.String())
	res, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode == 409 && res.Header.Get("X-Line-Accepted-Request-Id") != "" {
		return nil
	}
	if res.StatusCode != 200 {
		return fmt.Errorf("LINE returned HTTP %d", res.StatusCode)
	}
	return nil
}
