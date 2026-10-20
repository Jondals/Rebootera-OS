// * Prueba del resolvedor de Microsoft con servidores HTTPS falsos.
package catalog

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"rebootera/core/internal/testutil"
)

// microsoftSite imita la API de descargas de Microsoft: sesión, SKU y enlaces.
func microsoftSite(link string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/tags", func(w http.ResponseWriter, r *http.Request) {})
	mux.HandleFunc("/mdt.js", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`window.dfp={url:"https://` + r.Host + `/frame?session_id=abc&w=1F"};src+="&rticks="+12345;`))
	})
	mux.HandleFunc("/frame", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("rticks") != "12345" || r.URL.Query().Get("mdt") == "" {
			http.Error(w, "falta", http.StatusBadRequest)
		}
	})
	mux.HandleFunc("/api/getskuinformationbyproductedition", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"Skus":[{"Id":"1","Language":"English"},{"Id":"2","Language":"Spanish"}]}`))
	})
	mux.HandleFunc("/api/GetProductDownloadLinksBySku", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("SKU") != "2" || r.Header.Get("Referer") == "" {
			w.Write([]byte(`{"Errors":[{"Value":"Sentinel"}]}`))
			return
		}
		w.Write([]byte(`{"ProductDownloadOptions":[{"Uri":"` + link + `/x86/Win_Spanish_x86.iso?t=1","DownloadType":0},{"Uri":"` + link + `/Win_Spanish_x64.iso?t=1","DownloadType":1}]}`))
	})
	return mux
}

// * TestMicrosoft comprueba que se pide el idioma elegido y se queda el enlace x64 de un dominio de Microsoft.
func TestMicrosoft(t *testing.T) {
	server := httptest.NewTLSServer(nil)
	defer server.Close()
	server.Config.Handler = microsoftSite(server.URL)
	t.Cleanup(testutil.Keep(&Client).Restore)
	t.Cleanup(testutil.Keep(&msSessionURL).Restore)
	t.Cleanup(testutil.Keep(&msScriptURL).Restore)
	t.Cleanup(testutil.Keep(&msAPIURL).Restore)
	t.Cleanup(testutil.Keep(&msHostSuffix).Restore)
	Client = server.Client()
	msSessionURL = server.URL + "/tags?session_id="
	msScriptURL = server.URL + "/mdt.js?session_id="
	msAPIURL = server.URL + "/api/"
	msHostSuffix = ""

	got, err := resolveMicrosoft(Resolver{Type: "microsoft", Edition: "3321", Language: "Spanish"}, "https://www.microsoft.com/software-download/windows11")
	if err != nil {
		t.Fatal(err)
	}
	if got.FileName != "Win_Spanish_x64.iso" || !strings.HasPrefix(got.URL, server.URL) || got.SHA256 != "" {
		t.Fatalf("resultado: %+v", got)
	}
	if _, err := resolveMicrosoft(Resolver{Type: "microsoft", Edition: "3321", Language: "Klingon"}, "https://x"); err == nil {
		t.Fatal("un idioma que Microsoft no ofrece no debería resolverse")
	}
}

// * TestMicrosoftEnlaceAjeno comprueba que un enlace que no es de Microsoft o no es una ISO se descarta.
func TestMicrosoftEnlaceAjeno(t *testing.T) {
	for _, link := range []string{
		"https://evil.example.com/Win.iso",
		"http://software.download.prss.microsoft.com/Win.iso",
		"https://software.download.prss.microsoft.com/Win.exe",
	} {
		if _, err := msResolved(link); err == nil {
			t.Fatalf("%s debería descartarse", link)
		}
	}
	if got, err := msResolved("https://software.download.prss.microsoft.com/dbazure/Win11.iso?t=1"); err != nil || got.FileName != "Win11.iso" {
		t.Fatalf("enlace bueno: %+v %v", got, err)
	}
}
