import { computed, inject, Injectable, signal } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Router, UrlTree } from '@angular/router';
import { catchError, finalize, map, Observable, of, shareReplay, switchMap, tap } from 'rxjs';
import { environment } from '../../../environments/environment';
import { Sesion, TokenResponse } from '../models/api.models';

@Injectable({ providedIn: 'root' })
export class AuthService {
  private readonly http = inject(HttpClient);
  private readonly router = inject(Router);
  private readonly tokenState = signal<string | null>(null);
  private readonly sessionState = signal<Sesion | null>(null);
  private refreshRequest?: Observable<TokenResponse>;

  readonly token = computed(() => this.tokenState());
  readonly session = computed(() => this.sessionState());
  readonly empresaActiva = computed(() => {
    const actual = this.sessionState();
    return actual?.empresas.find((empresa) => empresa.id === actual.empresaActivaId) ?? null;
  });
  readonly autenticado = computed(() => this.tokenState() !== null);

  login(correo: string, contrasena: string): Observable<Sesion> {
    return this.http.post<TokenResponse>(`${environment.apiUrl}/auth/login`, { correo, contrasena }).pipe(
      tap((respuesta) => this.tokenState.set(respuesta.accessToken)), switchMap(() => this.cargarSesion())
    );
  }

  registrar(nombre: string, apellido: string, correo: string, contrasena: string): Observable<Sesion> {
    return this.http.post<TokenResponse>(`${environment.apiUrl}/auth/register`, { nombre, apellido, correo, contrasena }).pipe(
      tap((respuesta) => this.tokenState.set(respuesta.accessToken)), switchMap(() => this.cargarSesion())
    );
  }

  cargarSesion(): Observable<Sesion> {
    return this.http.get<Sesion>(`${environment.apiUrl}/auth/me`).pipe(tap((sesion) => this.sessionState.set(sesion)));
  }

  refresh(): Observable<TokenResponse> {
    if (this.refreshRequest) return this.refreshRequest;
    this.refreshRequest = this.http.post<TokenResponse>(`${environment.apiUrl}/auth/refresh`, {}, { withCredentials: true }).pipe(
      tap((respuesta) => this.tokenState.set(respuesta.accessToken)),
      finalize(() => { this.refreshRequest = undefined; }), shareReplay({ bufferSize: 1, refCount: false })
    );
    return this.refreshRequest;
  }

  asegurarSesion(): Observable<boolean | UrlTree> {
    const restaurar = this.tokenState()
      ? this.cargarSesion()
      : this.refresh().pipe(switchMap(() => this.cargarSesion()));
    return restaurar.pipe(
      map((): boolean | UrlTree => true),
      catchError(() => of(this.router.createUrlTree(['/acceso'])))
    );
  }

  seleccionarEmpresa(empresaId: string): Observable<void> {
    return this.http.post<TokenResponse>(`${environment.apiUrl}/auth/select-company`, { empresaId }).pipe(
      tap((respuesta) => this.tokenState.set(respuesta.accessToken)), switchMap(() => this.cargarSesion()), map(() => void 0)
    );
  }

  cerrarSesion(): void {
    this.http.post<void>(`${environment.apiUrl}/auth/logout`, {}, { withCredentials: true }).pipe(catchError(() => of(void 0))).subscribe();
    this.tokenState.set(null); this.sessionState.set(null); void this.router.navigate(['/acceso']);
  }

  limpiarSesion(): void { this.tokenState.set(null); this.sessionState.set(null); }
}

