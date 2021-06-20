package main

import (
	"encoding/base32"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
	"io/ioutil"
	"net/http"
	"strings"
)

type googleAuthHandler struct {
	config *oauth2.Config

	client *http.Client
}

func newGoogleAuthHandler(credentialPath string) (*googleAuthHandler, error) {
	b, err := ioutil.ReadFile(credentialPath)
	if err != nil {
		return nil, err
	}

	config, err := google.ConfigFromJSON(b, calendar.CalendarScope)
	if err != nil {
		return nil, err
	}

	// sample init http client directly
	tokenString := `{"access_token":"ya29.a0AfH6SMDJCCrc4_Hj-95INCEDqUN9ogtJzxPuv_eSDNp9wifAnpU-J6ORF2R8QBUM68HfUD2ZTgrofTScwRoitev9JFFyFHlzzKlY8nu0I0YffQ9wFzOVqY1_Bix_Ztd3P1cl0yO6RliGhkllyNxyDRt1W0xl","expire":"2021-06-18T23:38:44.722788+07:00","refresh_token":"","token_type":"Bearer"}`
	token := &oauth2.Token{}
	err = json.NewDecoder(strings.NewReader(tokenString)).Decode(token)
	if err != nil {
		return nil, err
	}
	//

	return &googleAuthHandler{config: config, client: config.Client(context.Background(), token)}, nil
}

func (h *googleAuthHandler) handleGetAuthURL(c *gin.Context) {
	authURL := h.config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	c.JSON(http.StatusOK, map[string]interface{}{
		"auth_url": authURL,
	})
}

func (h *googleAuthHandler) handleOAuthRedirectToken(c *gin.Context) {
	authCode := c.Query("code")
	token, err := h.config.Exchange(context.Background(), authCode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	h.client = h.config.Client(context.Background(), token)

	c.JSON(http.StatusOK, map[string]interface{}{
		"token_type":    token.TokenType,
		"access_token":  token.AccessToken,
		"refresh_token": token.RefreshToken,
		"expire":        token.Expiry,
	})
}

func (h *googleAuthHandler) createNewCalendar(c *gin.Context) {
	if h.client == nil {
		c.JSON(http.StatusBadRequest, "authenticate first")
		return
	}

	calService, err := calendar.NewService(context.Background(), option.WithHTTPClient(h.client))
	if err != nil {
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	res, err := calService.Calendars.Insert(&calendar.Calendar{
		Description: "Google calendar for Ohana customer",
		Summary:     "Ohana Customer",
	}).Do()
	if err != nil {
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"calendar_id": res.Id,
	})
}

func (h *googleAuthHandler) getCalendars(c *gin.Context) {
	if h.client == nil {
		c.JSON(http.StatusBadRequest, "authenticate first")
		return
	}

	calService, err := calendar.NewService(context.Background(), option.WithHTTPClient(h.client))
	if err != nil {
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	res, err := calService.CalendarList.List().Do()
	if err != nil {
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, res.Items)
}

func (h *googleAuthHandler) createEvent(c *gin.Context) {
	if h.client == nil {
		c.JSON(http.StatusBadRequest, "authenticate first")
		return
	}

	var req = struct {
		CalendarID  string `json:"calendar_id"`
		Title       string `json:"title"`
		Location    string `json:"location"`
		Description string `json:"description"`
		StartDate   string `json:"start_date"`
		EndDate     string `json:"end_date"`
	}{}
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, err.Error())
		return
	}

	calService, err := calendar.NewService(context.Background(), option.WithHTTPClient(h.client))
	if err != nil {
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	uuidEventID, err := uuid.NewV4()
	if err != nil {
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}
	eventID := base32.HexEncoding.WithPadding(base32.NoPadding).EncodeToString(uuidEventID.Bytes())
	eventID = strings.ToLower(eventID)

	// Same as update
	res, err := calService.Events.Insert(req.CalendarID, &calendar.Event{
		Id:          eventID,
		Summary:     req.Title,
		Location:    req.Location,
		Description: req.Description,
		Recurrence:  []string{"RRULE:FREQ=DAILY"},
		Start: &calendar.EventDateTime{
			Date:     req.StartDate,
			TimeZone: "Asia/Jakarta",
		},
		End: &calendar.EventDateTime{
			Date:     req.EndDate,
			TimeZone: "Asia/Jakarta",
		},
		Reminders: &calendar.EventReminders{
			UseDefault: true,
			Overrides: []*calendar.EventReminder{
				{Method: "popup", Minutes: 60},
			},
			ForceSendFields: []string{"UseDefault"},
		},
	}).Do()
	if err != nil {
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, res)
}

func main() {
	handler, err := newGoogleAuthHandler("credentials.json")
	if err != nil {
		panic(err)
	}

	r := gin.Default()
	r.GET("/api/google/calendars/auth", handler.handleGetAuthURL)
	r.GET("/api/google/calendars/token", handler.handleOAuthRedirectToken)
	r.POST("/api/calendars", handler.createNewCalendar)
	r.GET("/api/calendars", handler.getCalendars)
	r.POST("/api/calendars/events", handler.createEvent)
	if err := r.Run(); err != nil {
		panic(err)
	}
}
