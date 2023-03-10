package main

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/harryng22/moviedb/internal/data"
	"github.com/harryng22/moviedb/internal/validator"
)

type Input struct {
	Title   *string       `json:"title"`
	Year    *int32        `json:"year"`
	Runtime *data.Runtime `json:"runtime"`
	Genres  []string      `json:"genres"`
}

func (app *application) createMovieHandler(w http.ResponseWriter, r *http.Request) {
	var input Input

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	movie := &data.Movie{
		Title:   *input.Title,
		Year:    *input.Year,
		Runtime: *input.Runtime,
		Genres:  input.Genres,
	}

	// Validation
	v := validator.New()

	if data.ValidateMovie(v, movie); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// Insert to db
	err = app.model.Movie.Insert(movie)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	headers := make(http.Header)
	headers.Set("Location", fmt.Sprintf("/v1/movies/%d", movie.ID))

	err = app.writeJSON(w, http.StatusCreated, envelope{"movie": movie}, headers)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) showMovieHandler(w http.ResponseWriter, r *http.Request) {
	// Parse Id
	id, err := app.readIdParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	// Fetch existing movie by Id
	movie, err := app.model.Movie.Get(id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"movie": movie}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) updateMovieHandler(w http.ResponseWriter, r *http.Request) {
	// Parse Id
	id, err := app.readIdParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	// Fetch existing movie by Id
	movie, err := app.model.Movie.Get(id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	if expectedVersion := r.Header.Get("X-Expected-Version"); expectedVersion != "" && expectedVersion != strconv.FormatInt(int64(movie.Version), 32) {
		app.editConflictResponse(w, r)
		return
	}

	// Read JSON to input
	var input Input

	err = app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
	}

	// Copy values from request body to movie
	copyProperties(input, movie)

	// Validate movie to update
	v := validator.New()
	if data.ValidateMovie(v, movie); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// Update movie
	err = app.model.Movie.Update(movie)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrEditConflict):
			app.editConflictResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"movie": movie}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) deleteMovieHandler(w http.ResponseWriter, r *http.Request) {
	// Parse Id
	id, err := app.readIdParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	err = app.model.Movie.Delete(id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	// For machine
	// err = app.writeJSON(w, http.StatusNoContent, nil, nil)

	// For human
	err = app.writeJSON(w, http.StatusOK, envelope{"message": "movie successfully deleted"}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) listMoviesHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Title  string
		Genres []string
		data.Filter
	}

	v := validator.New()

	queryString := r.URL.Query()

	input.Title = app.readString(queryString, "title", "")
	input.Genres = app.readCSV(queryString, "genres", []string{})
	input.Filter.Page = app.readInt(queryString, "page", 1, v)
	input.Filter.PageSize = app.readInt(queryString, "page_size", 20, v)
	input.Filter.Sort = app.readString(queryString, "sort", "id")
	input.Filter.SortSafeList = []string{"id", "title", "year", "runtime", "-id", "-title", "-year", "-runtime"}

	if data.ValidateFilter(v, input.Filter); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	movies, metadata, err := app.model.Movie.GetAll(input.Title, input.Genres, input.Filter)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	env := envelope{
		"metadata": metadata,
		"movies":   movies,
	}
	err = app.writeJSON(w, http.StatusOK, env, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
