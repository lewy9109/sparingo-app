package web

import (
	"net/http"
	"strings"
	"time"

	"sqoush-app/internal/model"

	"github.com/google/uuid"
)

func (s *Server) handleReportNew(w http.ResponseWriter, r *http.Request) {
	currentUser := s.currentUser(r)
	view := struct {
		BaseView
		Form ReportFormView
	}{
		BaseView: BaseView{
			Title:           "Zgłoś",
			CurrentUser:     currentUser,
			Users:           s.store.ListUsers(),
			IsAuthenticated: currentUser.ID != "",
			IsDev:           isDevMode(),
		},
		Form: ReportFormView{},
	}
	if err := s.templates.Render(w, "reports_new.html", view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleReportCreate(w http.ResponseWriter, r *http.Request) {
	currentUser := s.currentUser(r)
	if currentUser.ID == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "nieprawidłowe dane", http.StatusBadRequest)
		return
	}
	rType := strings.TrimSpace(r.FormValue("type"))
	title := strings.TrimSpace(r.FormValue("title"))
	description := strings.TrimSpace(r.FormValue("description"))
	if rType != string(model.ReportBug) && rType != string(model.ReportFeature) {
		s.renderReportFormError(w, currentUser, "Wybierz typ zgłoszenia.", rType, title, description)
		return
	}
	if title == "" || description == "" {
		s.renderReportFormError(w, currentUser, "Wypełnij tytuł i opis.", rType, title, description)
		return
	}
	report := model.Report{
		ID:          uuid.NewString(),
		UserID:      currentUser.ID,
		Type:        model.ReportType(rType),
		Title:       title,
		Description: description,
		Status:      model.ReportOpen,
		CreatedAt:   time.Now(),
	}
	if _, err := s.store.CreateReport(report); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/?notice=report_added", http.StatusSeeOther)
}

func (s *Server) renderReportFormError(w http.ResponseWriter, currentUser model.User, message string, rType string, title string, description string) {
	view := struct {
		BaseView
		Form ReportFormView
	}{
		BaseView: BaseView{
			Title:           "Zgłoś",
			CurrentUser:     currentUser,
			Users:           s.store.ListUsers(),
			IsAuthenticated: currentUser.ID != "",
			IsDev:           isDevMode(),
		},
		Form: ReportFormView{
			Type:        rType,
			Title:       title,
			Description: description,
			Error:       message,
		},
	}
	if err := s.templates.Render(w, "reports_new.html", view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleReportsList(w http.ResponseWriter, r *http.Request) {
	currentUser := s.currentUser(r)
	if !isSuperAdmin(currentUser) {
		http.Error(w, "brak uprawnień", http.StatusForbidden)
		return
	}
	reports := s.store.ListReports()
	items := make([]ReportListItem, 0, len(reports))
	for _, report := range reports {
		user, _ := s.store.GetUser(report.UserID)
		items = append(items, ReportListItem{Report: report, Reporter: user})
	}
	view := ReportsView{
		BaseView: BaseView{
			Title:           "Zgłoszenia",
			CurrentUser:     currentUser,
			Users:           s.store.ListUsers(),
			IsAuthenticated: true,
			IsDev:           isDevMode(),
		},
		Items: items,
	}
	if err := s.templates.Render(w, "reports_list.html", view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
