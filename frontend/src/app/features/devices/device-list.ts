import { DatePipe } from '@angular/common';
import { Component, computed, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { Router } from '@angular/router';
import { ButtonModule } from '@openng/optimus-ui/button';
import { CardModule } from '@openng/optimus-ui/card';
import { SelectModule } from '@openng/optimus-ui/select';
import { TableModule } from '@openng/optimus-ui/table';
import { TagModule } from '@openng/optimus-ui/tag';
import { TooltipModule } from '@openng/optimus-ui/tooltip';

import { Device } from '../../api';
import {
  SOURCE_IDS,
  commandsFor,
  isControllable,
  kindIcon,
  kindLabel,
  sourceLabel,
  sourceSeverity,
} from '../../core/device-state';
import { DevicesStore } from '../../core/devices.store';

@Component({
  selector: 'app-device-list',
  imports: [
    DatePipe,
    FormsModule,
    TableModule,
    ButtonModule,
    TagModule,
    SelectModule,
    CardModule,
    TooltipModule,
  ],
  templateUrl: './device-list.html',
})
export class DeviceList {
  private readonly router = inject(Router);
  protected readonly store = inject(DevicesStore);

  protected readonly kindLabel = kindLabel;
  protected readonly kindIcon = kindIcon;
  protected readonly isControllable = isControllable;
  protected readonly commandsFor = commandsFor;
  protected readonly sourceSeverity = sourceSeverity;

  protected readonly sourceFilter = signal<string | null>(null);
  protected readonly roomFilter = signal<string | null>(null);

  protected readonly sourceOptions = [
    { label: 'Toutes les sources', value: null },
    ...SOURCE_IDS.map((id) => ({ label: sourceLabel(id), value: id })),
  ];

  protected readonly roomOptions = computed(() => [
    { label: 'Toutes les pièces', value: null },
    ...this.store.rooms().map((room) => ({ label: room, value: room })),
  ]);

  /**
   * Le filtrage est fait côté client : l'API accepte les mêmes filtres, mais
   * un intérieur domestique compte quelques dizaines d'équipements au plus,
   * déjà tous chargés — un aller-retour réseau par changement de filtre
   * n'apporterait qu'une latence.
   */
  protected readonly filtered = computed(() => {
    const source = this.sourceFilter();
    const room = this.roomFilter();

    return this.store.devices().filter((device) => {
      if (source && device.source !== source) {
        return false;
      }
      if (room && device.room !== room) {
        return false;
      }
      return true;
    });
  });

  protected open(device: Device): void {
    void this.router.navigate(['/devices', device.id]);
  }

  protected send(event: Event, device: Device, command: string): void {
    // La ligne entière ouvre le détail ; empêcher la propagation garde le clic
    // sur un bouton de commande local à ce bouton.
    event.stopPropagation();
    this.store.sendCommand(device, command);
  }
}
