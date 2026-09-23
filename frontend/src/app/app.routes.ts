import { Routes } from '@angular/router';

export const routes: Routes = [
  { path: '', pathMatch: 'full', redirectTo: 'dashboard' },
  {
    path: 'dashboard',
    title: 'Tableau de bord — Domotic',
    loadComponent: () => import('./features/dashboard/dashboard').then((m) => m.Dashboard),
  },
  {
    path: 'devices',
    title: 'Équipements — Domotic',
    loadComponent: () => import('./features/devices/device-list').then((m) => m.DeviceList),
  },
  {
    // withComponentInputBinding() injecte `id` dans l'entrée du composant.
    path: 'devices/:id',
    title: 'Équipement — Domotic',
    loadComponent: () => import('./features/devices/device-detail').then((m) => m.DeviceDetail),
  },
  { path: '**', redirectTo: 'dashboard' },
];
