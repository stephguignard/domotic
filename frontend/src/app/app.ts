import { Component, computed, inject } from '@angular/core';
import { RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { ButtonModule } from '@openng/optimus-ui/button';
import { ConfirmDialogModule } from '@openng/optimus-ui/confirmdialog';
import { TagModule } from '@openng/optimus-ui/tag';
import { ToastModule } from '@openng/optimus-ui/toast';
import { ToolbarModule } from '@openng/optimus-ui/toolbar';

import { DevicesStore } from './core/devices.store';

@Component({
  selector: 'app-root',
  imports: [
    RouterOutlet,
    RouterLink,
    RouterLinkActive,
    ToolbarModule,
    ButtonModule,
    ConfirmDialogModule,
    TagModule,
    ToastModule,
  ],
  templateUrl: './app.html',
  styleUrl: './app.css',
})
export class App {
  protected readonly store = inject(DevicesStore);

  /** Résumé de l'état du service, affiché en permanence dans la barre. */
  protected readonly healthTag = computed(() => {
    const health = this.store.health();
    if (!health) {
      return { label: 'Hors ligne', severity: 'danger' as const };
    }
    if (health.status === 'ok') {
      return { label: 'Opérationnel', severity: 'success' as const };
    }

    // Nommer les sources en échec évite d'avoir à ouvrir /api/health pour
    // comprendre ce qui ne va pas.
    const failing = Object.entries(health.sources)
      .filter(([, s]) => s.enabled && !s.healthy)
      .map(([name]) => name);

    return {
      label: failing.length ? `Dégradé : ${failing.join(', ')}` : 'Dégradé',
      severity: 'warn' as const,
    };
  });

  constructor() {
    this.store.refresh();
  }
}
