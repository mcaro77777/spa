import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { ReactiveFormsModule, FormBuilder, Validators } from '@angular/forms';
import { Router } from '@angular/router';
import { HttpErrorResponse } from '@angular/common/http';
import { finalize } from 'rxjs';
import { AuthService } from '../../core/auth/auth.service';
import { ProblemaApi } from '../../core/models/api.models';

@Component({
  selector: 'app-access', standalone: true, imports: [ReactiveFormsModule],
  templateUrl: './access.component.html', styleUrl: './access.component.scss', changeDetection: ChangeDetectionStrategy.OnPush
})
export class AccessComponent {
  private readonly fb = inject(FormBuilder); private readonly auth = inject(AuthService); private readonly router = inject(Router);
  readonly modo = signal<'login'|'registro'>('login'); readonly cargando = signal(false); readonly error = signal(''); readonly mostrarClave = signal(false);
  readonly formulario = this.fb.nonNullable.group({
    nombre: [''], apellido: [''], correo: ['', [Validators.required, Validators.email]], contrasena: ['', [Validators.required, Validators.minLength(10)]]
  });

  cambiarModo(modo: 'login'|'registro'): void { this.modo.set(modo); this.error.set(''); }
  enviar(): void {
    if (this.formulario.invalid) { this.formulario.markAllAsTouched(); return; }
    const v = this.formulario.getRawValue(); this.cargando.set(true); this.error.set('');
    const solicitud = this.modo() === 'login' ? this.auth.login(v.correo, v.contrasena) : this.auth.registrar(v.nombre, v.apellido, v.correo, v.contrasena);
    solicitud.pipe(finalize(() => this.cargando.set(false))).subscribe({
      next: (sesion) => void this.router.navigate([sesion.empresaActivaId ? '/app/dashboard' : '/empresas']),
      error: (e: HttpErrorResponse) => this.error.set((e.error as ProblemaApi | undefined)?.detail || 'No fue posible completar el acceso.')
    });
  }
}

