import { HttpErrorResponse, HttpInterceptorFn } from '@angular/common/http';
import { inject } from '@angular/core';
import { catchError, switchMap, throwError } from 'rxjs';
import { AuthService } from './auth.service';

export const authInterceptor: HttpInterceptorFn = (request, next) => {
  const auth = inject(AuthService);
  const token = auth.token();
  const esAuth = request.url.includes('/auth/login') || request.url.includes('/auth/register') || request.url.includes('/auth/refresh');
  const autenticada = token ? request.clone({ setHeaders: { Authorization: `Bearer ${token}` } }) : request;
  return next(autenticada).pipe(catchError((error: HttpErrorResponse) => {
    if (error.status !== 401 || esAuth) return throwError(() => error);
    return auth.refresh().pipe(switchMap((respuesta) => next(request.clone({ setHeaders: { Authorization: `Bearer ${respuesta.accessToken}` } }))));
  }));
};

