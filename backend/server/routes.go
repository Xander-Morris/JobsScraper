package server

import "net/http"

func registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/health", handleHealth)
	mux.HandleFunc("GET /api/jobs", handleSearchJobs)
	mux.HandleFunc("GET /api/jobs/{id}", handleGetJob)
	mux.HandleFunc("GET /api/tags", handleGetTags)
	mux.HandleFunc("POST /api/profile/create", handleCreateProfile)
	mux.HandleFunc("POST /api/profile/login", handleLoginProfile)
	mux.HandleFunc("POST /api/profile/refresh", handleRefreshProfile)
	mux.HandleFunc("POST /api/profile/logout", handleLogoutProfile)
	mux.HandleFunc("GET /api/profile", withAuth(handleGetProfile))
	mux.HandleFunc("PUT /api/profile", withAuth(handleUpdateProfile))
	mux.HandleFunc("POST /api/profile/education", withAuth(handleAddEducation))
	mux.HandleFunc("DELETE /api/profile/education/{id}", withAuth(handleDeleteEducation))
	mux.HandleFunc("POST /api/profile/skills", withAuth(handleAddSkill))
	mux.HandleFunc("DELETE /api/profile/skills/{id}", withAuth(handleDeleteSkill))
	mux.HandleFunc("POST /api/profile/work-experience", withAuth(handleAddWorkExperience))
	mux.HandleFunc("DELETE /api/profile/work-experience/{id}", withAuth(handleDeleteWorkExperience))
	mux.HandleFunc("POST /api/profile/work-experience/{workExperienceId}/bullets", withAuth(handleAddWorkExperienceBullet))
	mux.HandleFunc("DELETE /api/profile/work-experience/{workExperienceId}/bullets/{id}", withAuth(handleDeleteWorkExperienceBullet))
	mux.HandleFunc("POST /api/profile/resumes", withAuth(handleUploadResume))
	mux.HandleFunc("PUT /api/profile/resumes/{id}", withAuth(handleUpdateResume))
	mux.HandleFunc("GET /api/profile/resumes/{id}/download", withAuth(handleDownloadResume))
	mux.HandleFunc("DELETE /api/profile/resumes/{id}", withAuth(handleDeleteResume))
	mux.HandleFunc("GET /api/profile/resumes/{id}/extraction", withAuth(handleGetResumeExtraction))
	mux.HandleFunc("POST /api/profile/resumes/{id}/extraction", withAuth(handleTriggerResumeExtraction))
}
