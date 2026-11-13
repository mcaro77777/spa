import { inject } from '@angular/core';
import { CanActivateFn, Router } from '@angular/router';
import { map } from 'rxjs';
import { AuthService } from './auth.service';

export const authGuard: CanActivateFn = () => inject(AuthService).asegurarSesion();

export const companyGuard: CanActivateFn = () => {
  const auth = inject(AuthService);
  const router = inject(Router);
  return auth.asegurarSesion().pipe(map((resultado) => resultado === true && !auth.session()?.empresaActivaId ? router.createUrlTree(['/empresas']) : resultado));
};
