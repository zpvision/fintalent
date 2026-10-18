package main

import (
	"FinTalent/internal/academy"
	"net/http"
)

func resolveAcademyAdministrator(r *http.Request) (academy.AdminPrincipal, error) {
	if isAdmin(r) {
		return academy.AdminPrincipal{Key: "platform_admin", Name: "Администратор FinTalent", Role: "superadmin", Contacts: true, Answers: true, Correct: true}, nil
	}
	u, err := userFromRequest(r)
	if err != nil {
		return academy.AdminPrincipal{}, err
	}
	return academy.ResolveAdminGrant(r.Context(), db, u.ID)
}
