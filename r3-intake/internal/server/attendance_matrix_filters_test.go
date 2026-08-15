package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestParseMatrixFiltersEventScopedDates(t *testing.T) {
	now := time.Now().In(hst)
	defTo := now.Format("2006-01-02")
	defFrom := now.AddDate(0, 0, -13).Format("2006-01-02")

	ev1 := Event{ID: "ev1", Name: "Event 1", StartDate: "2026-01-05", EndDate: "2026-01-10"}
	ev2 := Event{ID: "ev2", Name: "Event 2", StartDate: "2026-04-01", EndDate: "2026-04-07"}
	evLong := Event{ID: "evLong", Name: "Long Event", StartDate: "2026-01-01", EndDate: "2026-03-15"}
	evBad := Event{ID: "evBad", Name: "Bad Dates", StartDate: "", EndDate: ""}
	evInverted := Event{ID: "evInv", Name: "Inverted", StartDate: "2026-01-10", EndDate: "2026-01-05"}

	tests := []struct {
		name        string
		query       map[string]string
		events      []Event
		wantFrom    string
		wantTo      string
		wantEventID string
		wantLen     int
	}{
		{
			name:        "auto-scope to selected event span",
			query:       map[string]string{"event": "ev1"},
			events:      []Event{ev1},
			wantFrom:    "2026-01-05",
			wantTo:      "2026-01-10",
			wantEventID: "ev1",
			wantLen:     6,
		},
		{
			name:        "no events falls back to 14-day default",
			query:       nil,
			events:      nil,
			wantFrom:    defFrom,
			wantTo:      defTo,
			wantEventID: "",
			wantLen:     14,
		},
		{
			name:        "invalid event dates fall back to default",
			query:       map[string]string{"event": "evBad"},
			events:      []Event{evBad},
			wantFrom:    defFrom,
			wantTo:      defTo,
			wantEventID: "evBad",
			wantLen:     14,
		},
		{
			name:        "inverted event dates fall back to default",
			query:       map[string]string{"event": "evInv"},
			events:      []Event{evInverted},
			wantFrom:    defFrom,
			wantTo:      defTo,
			wantEventID: "evInv",
			wantLen:     14,
		},
		{
			name:        "explicit from/to wins over event dates",
			query:       map[string]string{"event": "ev1", "from": "2026-02-01", "to": "2026-02-05"},
			events:      []Event{ev1},
			wantFrom:    "2026-02-01",
			wantTo:      "2026-02-05",
			wantEventID: "ev1",
			wantLen:     5,
		},
		{
			name:        "event span longer than 30 days is not capped",
			query:       map[string]string{"event": "evLong"},
			events:      []Event{evLong},
			wantFrom:    "2026-01-01",
			wantTo:      "2026-03-15",
			wantEventID: "evLong",
			wantLen:     74,
		},
		{
			name:        "query event key wins over event_id",
			query:       map[string]string{"event": "ev1", "event_id": "ev2"},
			events:      []Event{ev1, ev2},
			wantFrom:    "2026-01-05",
			wantTo:      "2026-01-10",
			wantEventID: "ev1",
			wantLen:     6,
		},
		{
			name:        "event_id used when event absent",
			query:       map[string]string{"event_id": "ev2"},
			events:      []Event{ev1, ev2},
			wantFrom:    "2026-04-01",
			wantTo:      "2026-04-07",
			wantEventID: "ev2",
			wantLen:     7,
		},
		{
			name:        "defaults to first event and auto-scopes",
			query:       nil,
			events:      []Event{ev1, ev2},
			wantFrom:    "2026-01-05",
			wantTo:      "2026-01-10",
			wantEventID: "ev1",
			wantLen:     6,
		},
		{
			name:        "partial explicit range still auto-scopes to event",
			query:       map[string]string{"event": "ev1", "from": "2026-02-01"},
			events:      []Event{ev1},
			wantFrom:    "2026-01-05",
			wantTo:      "2026-01-10",
			wantEventID: "ev1",
			wantLen:     6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := &Server{}
			req := httptest.NewRequest(http.MethodGet, "/matrix", nil)
			q := req.URL.Query()
			for k, v := range tt.query {
				q.Set(k, v)
			}
			req.URL.RawQuery = q.Encode()

			from, to, eventID, dates := srv.parseMatrixFilters(req, tt.events)

			if from != tt.wantFrom {
				t.Errorf("from = %q, want %q", from, tt.wantFrom)
			}
			if to != tt.wantTo {
				t.Errorf("to = %q, want %q", to, tt.wantTo)
			}
			if eventID != tt.wantEventID {
				t.Errorf("eventID = %q, want %q", eventID, tt.wantEventID)
			}
			if len(dates) != tt.wantLen {
				t.Errorf("len(dates) = %d, want %d; dates=%v", len(dates), tt.wantLen, dates)
			}
			for _, d := range dates {
				if _, err := time.Parse("2006-01-02", d); err != nil {
					t.Errorf("dates contains non-date %q: %v", d, err)
				}
			}
		})
	}
}
