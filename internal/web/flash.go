package web

import "strings"

func flashMessage(notice string) string {
	switch strings.TrimSpace(notice) {
	case "friendly_added":
		return "Dodano mecz towarzyski."
	case "report_added":
		return "Dziękujemy! Zgłoszenie zostało zapisane."
	case "join_requested":
		return "Wysłano prośbę o dołączenie do ligi."
	}
	return ""
}
