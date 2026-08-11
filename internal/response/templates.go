package response

import (
	"bytes"
	"html/template"
	"maps"
	"net/http"
	"net/url"

	"github.com/v8tix/beecore-store-v2/assets"
	"github.com/v8tix/beecore-store-v2/internal/funcs"
)

func Page(w http.ResponseWriter, status int, data any, pagePath string) error {
	return PageWithHeaders(w, status, data, nil, pagePath)
}

func PageRedirect(w http.ResponseWriter, r *http.Request, status int, data any, pagePath string, queryParams map[string]string) error {
	return PageWithHeadersAndQueryParams(w, r, status, data, nil, pagePath, queryParams)
}

func PageWithHeaders(
	w http.ResponseWriter,
	status int,
	data any,
	headers http.Header,
	pagePath string,
) error {
	patterns := []string{"base-tmpl-1.html", "partials/*.html", pagePath}

	return NamedTemplateWithHeaders(w, status, data, headers, "base", patterns...)
}

func PageWithHeadersAndQueryParams(
	w http.ResponseWriter,
	r *http.Request,
	status int,
	data any,
	headers http.Header,
	pagePath string,
	queryParams map[string]string,
) error {
	patterns := []string{"base-tmpl-1.html", "partials/*.html", pagePath}

	return NamedTemplateWithHeadersAndQueryParams(w, r, status, data, headers, "base", patterns, queryParams)
}

func NamedTemplate(
	w http.ResponseWriter,
	status int,
	data any,
	templateName string,
	patterns ...string,
) error {
	return NamedTemplateWithHeaders(w, status, data, nil, templateName, patterns...)
}

func NamedTemplateWithHeadersAndQueryParams(
	w http.ResponseWriter,
	r *http.Request,
	status int,
	data any,
	headers http.Header,
	templateName string,
	patterns []string,
	queryParams map[string]string,
) error {
	for i := range patterns {
		patterns[i] = "templates/" + patterns[i]
	}

	ts, err := template.New("").Funcs(funcs.TemplateFuncs).ParseFS(assets.EmbeddedFiles, patterns...)
	if err != nil {
		return err
	}

	buf := new(bytes.Buffer)

	err = ts.ExecuteTemplate(buf, templateName, data)
	if err != nil {
		return err
	}

	maps.Copy(w.Header(), headers)

	u, err := url.Parse(r.URL.String())
	if err != nil {
		return err
	}
	q := u.Query()
	for key, value := range queryParams {
		q.Set(key, value)
	}
	u.RawQuery = q.Encode()

	url := u.String()

	http.Redirect(w, r, url, status)
	//buf.WriteTo(w)

	return nil
}

func NamedTemplateWithHeaders(
	w http.ResponseWriter,
	status int,
	data any,
	headers http.Header,
	templateName string,
	patterns ...string,
) error {
	for i := range patterns {
		patterns[i] = "templates/" + patterns[i]
	}

	ts, err := template.New("").Funcs(funcs.TemplateFuncs).ParseFS(assets.EmbeddedFiles, patterns...)
	if err != nil {
		return err
	}

	buf := new(bytes.Buffer)

	err = ts.ExecuteTemplate(buf, templateName, data)
	if err != nil {
		return err
	}

	maps.Copy(w.Header(), headers)

	w.WriteHeader(status)
	buf.WriteTo(w)

	return nil
}
