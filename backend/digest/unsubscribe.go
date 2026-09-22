package digest

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"

	"main/utils"
)

// unsubscribeToken is an HMAC over the profile ID, keyed on the same
// SECRET_KEY as access-token JWTs. Lets the one-click unsubscribe link prove
// we minted it for this profile, without the recipient needing to log in.
func unsubscribeToken(profileID int64) string {
	mac := hmac.New(sha256.New, []byte(utils.GetEnv()["SECRET_KEY"]))
	mac.Write([]byte(strconv.FormatInt(profileID, 10)))

	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// ValidUnsubscribeToken reports whether token is the one this profile's
// unsubscribe link carries.
func ValidUnsubscribeToken(profileID int64, token string) bool {
	return hmac.Equal([]byte(unsubscribeToken(profileID)), []byte(token))
}

func unsubscribeLink(profileID int64) string {
	base := strings.TrimSuffix(os.Getenv("PUBLIC_BACKEND_URL"), "/")
	if base == "" {
		base = "http://localhost:8090"
	}

	return fmt.Sprintf("%s/api/digest/unsubscribe?profile_id=%d&token=%s", base, profileID, unsubscribeToken(profileID))
}
