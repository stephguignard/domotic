import { DatePipe } from '@angular/common';
import { Component, effect, inject, input, signal } from '@angular/core';
import { RouterLink } from '@angular/router';
import { TableModule } from '@openng/optimus-ui/table';
import { TagModule } from '@openng/optimus-ui/tag';
import { TooltipModule } from '@openng/optimus-ui/tooltip';

import { CommandLogEntry, HistoryService } from '../../api';
import { actionLabel, kindIcon, sourceLabel, sourceSeverity } from '../../core/device-state';
import { DevicesStore } from '../../core/devices.store';

/**
 * Historique des actions envoyées depuis l'interface.
 *
 * Sans `deviceId`, toutes les actions, avec l'équipement visé ; avec, celles
 * d'un seul équipement, sans ces colonnes devenues redondantes. La liste se
 * relit après chaque commande envoyée depuis n'importe quelle page.
 */
@Component({
  selector: 'app-command-history',
  imports: [DatePipe, RouterLink, TableModule, TagModule, TooltipModule],
  templateUrl: './command-history.html',
})
export class CommandHistory {
  readonly deviceId = input<string>();
  readonly limit = input(200);

  private readonly historyApi = inject(HistoryService);
  private readonly store = inject(DevicesStore);

  protected readonly actionLabel = actionLabel;
  protected readonly kindIcon = kindIcon;
  protected readonly sourceLabel = sourceLabel;
  protected readonly sourceSeverity = sourceSeverity;

  protected readonly entries = signal<CommandLogEntry[]>([]);
  protected readonly loading = signal(true);
  protected readonly error = signal(false);

  constructor() {
    effect(() => {
      const deviceId = this.deviceId();
      const limit = this.limit();
      this.store.commandsSent(); // relire après chaque action
      this.load(deviceId, limit);
    });
  }

  private load(deviceId: string | undefined, limit: number): void {
    this.loading.set(true);
    this.historyApi.listCommands(deviceId, limit).subscribe({
      next: (response) => {
        this.entries.set(response.entries);
        this.error.set(false);
        this.loading.set(false);
      },
      error: () => {
        this.error.set(true);
        this.loading.set(false);
      },
    });
  }
}
