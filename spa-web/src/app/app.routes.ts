import { Routes } from '@angular/router';
import { authGuard, companyGuard } from './core/auth/auth.guard';

export const routes: Routes = [
  { path: '', pathMatch: 'full', redirectTo: 'app/dashboard' },
  { path: 'acceso', loadComponent: () => import('./features/auth/access.component').then((m) => m.AccessComponent) },
  { path: 'empresas', canActivate: [authGuard], loadComponent: () => import('./features/companies/companies.component').then((m) => m.CompaniesComponent) },
  {
    path: 'app', canActivate: [companyGuard], loadComponent: () => import('./layout/app-layout.component').then((m) => m.AppLayoutComponent),
    children: [
      { path: '', pathMatch: 'full', redirectTo: 'dashboard' },
      { path: 'dashboard', loadComponent: () => import('./features/dashboard/dashboard.component').then((m) => m.DashboardComponent) },
      { path: 'documentos', loadComponent: () => import('./features/documents/documents.component').then((m) => m.DocumentsComponent) },
      { path: 'importaciones', loadComponent: () => import('./features/imports/imports.component').then((m) => m.ImportsComponent) }
    ]
  },
  { path: '**', redirectTo: 'app/dashboard' }
];
