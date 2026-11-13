package httpapi

import (
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"spa-api/internal/configuracion"
	"spa-api/internal/version"
)

type Servidor struct { cfg configuracion.Configuracion; db *sql.DB; logger *slog.Logger }

func NuevoServidor(cfg configuracion.Configuracion, db *sql.DB, logger *slog.Logger) http.Handler {
	s:=&Servidor{cfg:cfg,db:db,logger:logger}; mux:=http.NewServeMux()
	identidad:=func(h http.HandlerFunc) http.Handler{return s.requerirSesion(false,h)}; empresa:=func(h http.HandlerFunc) http.Handler{return s.requerirSesion(true,h)}
	mux.HandleFunc("GET /api/health",s.salud); mux.HandleFunc("GET /api/ready",s.salud); mux.HandleFunc("GET /api/version",s.version)
	mux.HandleFunc("POST /api/auth/register",s.registrar); mux.HandleFunc("POST /api/auth/login",s.login); mux.HandleFunc("POST /api/auth/refresh",s.refrescar); mux.HandleFunc("POST /api/auth/logout",s.logout)
	mux.Handle("GET /api/auth/me",identidad(s.miSesion)); mux.Handle("POST /api/auth/select-company",identidad(s.seleccionarEmpresa))
	mux.Handle("GET /api/companies",identidad(s.empresas)); mux.Handle("POST /api/companies",identidad(s.empresas)); mux.Handle("GET /api/companies/{id}",empresa(s.empresaDetalle)); mux.Handle("PATCH /api/companies/{id}",empresa(s.empresaDetalle))
	mux.Handle("POST /api/imports/invoices",empresa(s.importarDocumentos)); mux.Handle("GET /api/imports",empresa(s.listarImportaciones)); mux.Handle("GET /api/imports/{id}",empresa(s.detalleImportacion)); mux.Handle("GET /api/imports/{id}/errors",empresa(s.erroresImportacion))
	mux.Handle("GET /api/invoices",empresa(s.listarDocumentos)); mux.Handle("GET /api/invoices/{id}",empresa(s.detalleDocumento))
	mux.Handle("GET /api/dashboard/summary",empresa(s.resumenDashboard)); mux.Handle("GET /api/tax/vat-estimate",empresa(s.estimacionIVA))
	mux.Handle("GET /api/reports/purchases-sales.xlsx",empresa(s.reporteComprasVentasExcel)); mux.Handle("GET /api/reports/vat-estimate.pdf",empresa(s.reporteIVAPDF))
	mux.HandleFunc("/api/",func(w http.ResponseWriter,r *http.Request){responderProblema(w,r,404,"ruta_no_encontrada","Ruta no encontrada","")})
	mux.Handle("/",manejadorSPA(cfg.DirectorioCliente))
	return middlewareBase(logger,cfg.URLBase,!cfg.Produccion(),mux)
}

func (s *Servidor) salud(w http.ResponseWriter,r *http.Request){ if err:=s.db.PingContext(r.Context());err!=nil{responderProblema(w,r,503,"base_no_disponible","Base de datos no disponible","");return}; responderJSON(w,200,map[string]any{"ok":true,"database":"ok","version":version.Version}) }
func (s *Servidor) version(w http.ResponseWriter,_ *http.Request){responderJSON(w,200,map[string]string{"version":version.Version,"commit":version.Commit})}

func manejadorSPA(dir string) http.Handler { indice:=filepath.Join(dir,"index.html"); archivos:=http.FileServer(http.Dir(dir)); return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){ if _,err:=os.Stat(indice);err!=nil{w.Header().Set("Content-Type","text/html; charset=utf-8");w.WriteHeader(503);_,_=w.Write([]byte("<!doctype html><html><body><h1>MiSpA API activa</h1><p>Falta compilar spa-web.</p></body></html>"));return}; rel:=strings.TrimPrefix(filepath.ToSlash(filepath.Clean("/"+r.URL.Path)),"/"); ruta:=filepath.Join(dir,filepath.FromSlash(rel)); if info,err:=os.Stat(ruta);err==nil&&!info.IsDir(){archivos.ServeHTTP(w,r);return};w.Header().Set("Cache-Control","no-cache");http.ServeFile(w,r,indice) }) }
