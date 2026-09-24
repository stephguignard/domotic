import { Injectable, computed, inject, signal } from '@angular/core';
import { ConfirmationService, MessageService } from '@openng/optimus-ui/api';

import { Device, DevicesService, HealthOutputBody, HealthService } from '../api';
import { commandsFor, needsConfirmation } from './device-state';

/**
 * État partagé des équipements.
 *
 * Le backend consolide déjà les sources : le frontend n'a qu'à relire
 * périodiquement cette vue unifiée. Un simple rafraîchissement suffit, sans
 * WebSocket ni SSE — les changements d'état domestiques ne se comptent pas en
 * millisecondes, et cela garde le service léger sur le NAS.
 */
@Injectable({ providedIn: 'root' })
export class DevicesStore {
  private readonly devicesApi = inject(DevicesService);
  private readonly healthApi = inject(HealthService);
  private readonly messages = inject(MessageService);
  private readonly confirmation = inject(ConfirmationService);

  private readonly devicesSignal = signal<Device[]>([]);
  private readonly healthSignal = signal<HealthOutputBody | null>(null);
  private readonly loadingSignal = signal(false);
  private readonly errorSignal = signal<string | null>(null);

  /** Équipements consolidés, toutes sources confondues. */
  readonly devices = this.devicesSignal.asReadonly();
  /** État du service et de ses sources. */
  readonly health = this.healthSignal.asReadonly();
  /** Un chargement est-il en cours ? */
  readonly loading = this.loadingSignal.asReadonly();
  /** Message de la dernière erreur de chargement, le cas échéant. */
  readonly error = this.errorSignal.asReadonly();

  /** Pièces représentées, triées. */
  readonly rooms = computed(() => {
    const names = new Set(this.devicesSignal().map((d) => d.room).filter((r) => r !== ''));
    return [...names].sort((a, b) => a.localeCompare(b, 'fr'));
  });

  /** Équipements groupés par pièce, les équipements sans pièce en dernier. */
  readonly byRoom = computed(() => {
    const groups = new Map<string, Device[]>();
    for (const device of this.devicesSignal()) {
      const key = device.room || 'Sans pièce';
      const group = groups.get(key);
      if (group) {
        group.push(device);
      } else {
        groups.set(key, [device]);
      }
    }
    return [...groups.entries()]
      .sort(([a], [b]) => {
        if (a === 'Sans pièce') return 1;
        if (b === 'Sans pièce') return -1;
        return a.localeCompare(b, 'fr');
      })
      .map(([room, devices]) => ({ room, devices }));
  });

  /** Nombre d'équipements injoignables. */
  readonly unreachableCount = computed(
    () => this.devicesSignal().filter((d) => !d.reachable).length,
  );

  /** Recharge les équipements et l'état du service. */
  refresh(): void {
    this.loadingSignal.set(true);

    this.devicesApi.listDevices().subscribe({
      next: (response) => {
        this.devicesSignal.set(response.devices);
        this.errorSignal.set(null);
        this.loadingSignal.set(false);
      },
      error: (err: unknown) => {
        this.errorSignal.set(describeError(err));
        this.loadingSignal.set(false);
      },
    });

    // L'état de santé est secondaire : son échec ne doit pas masquer les
    // équipements déjà chargés, d'où l'absence de message d'erreur ici.
    this.healthApi.health().subscribe({
      next: (health) => this.healthSignal.set(health),
      error: () => this.healthSignal.set(null),
    });
  }

  /**
   * Envoie une commande à un équipement, puis rafraîchit son état. Les
   * commandes sensibles passent d'abord par une confirmation.
   */
  sendCommand(device: Device, command: string, parameters: unknown[] = []): void {
    if (!needsConfirmation(device)) {
      this.execute(device, command, parameters);
      return;
    }

    const label = commandsFor(device.kind).find((c) => c.command === command)?.label ?? command;
    this.confirmation.confirm({
      header: `${label} « ${device.name} » ?`,
      message:
        'Cette commande agit sur une installation électrique. ' +
        "Si un interrupteur physique est relié au module, il reprendra la main à son prochain changement.",
      icon: 'pi pi-exclamation-triangle',
      acceptLabel: label,
      rejectLabel: 'Annuler',
      rejectButtonProps: { severity: 'secondary', outlined: true },
      accept: () => this.execute(device, command, parameters),
    });
  }

  private execute(device: Device, command: string, parameters: unknown[]): void {
    this.devicesApi.sendCommand(device.id, { command, parameters }).subscribe({
      next: () => {
        this.messages.add({
          severity: 'success',
          summary: device.name,
          detail: `Commande « ${command} » envoyée`,
        });

        // Les sources exécutent la commande de façon asynchrone ; laisser au
        // backend le temps de consolider le nouvel état avant de relire.
        setTimeout(() => this.refresh(), 3000);
      },
      error: (err: unknown) => {
        this.messages.add({
          severity: 'error',
          summary: device.name,
          detail: describeError(err),
          life: 8000,
        });
      },
    });
  }
}

/** Extrait un message lisible d'une erreur HTTP. */
function describeError(err: unknown): string {
  if (typeof err === 'object' && err !== null) {
    // Le backend renvoie du RFC 7807 : le champ `detail` porte le message utile.
    const body = (err as { error?: { detail?: string; title?: string } }).error;
    if (body?.detail) {
      return body.detail;
    }
    if (body?.title) {
      return body.title;
    }
    const message = (err as { message?: string }).message;
    if (message) {
      return message;
    }
  }
  return 'Erreur inattendue';
}
