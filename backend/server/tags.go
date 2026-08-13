package server

import (
	"fmt"
	"main/database"
	"net/http"
)

func handleGetTags(w http.ResponseWriter, r *http.Request) {
	res, err := database.FetchAllUniqueTags(r.Context())
	fmt.Println(res, err)

	if err != nil {
		writeJSON(w, http.StatusOK, res)
	} else {
		writeJSON(w, http.StatusInternalServerError, res)
	}
}