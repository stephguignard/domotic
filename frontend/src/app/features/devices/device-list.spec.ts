import { HttpTestingController } from '@angular/common/http/testing';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { Router } from '@angular/router';

import { DeviceList } from './device-list';
import { Device } from '../../api';
import { DevicesStore } from '../../core/devices.store';
import { flushRoomOrder, makeDevice, makeStation, testProviders } from '../../testing/providers';

describe('DeviceList', () => {
  let http: HttpTestingController;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [DeviceList],
      providers: testProviders(),
    }).compileComponents();

    http = TestBed.inject(HttpTestingController);
  });

  afterEach(() => http.verify());

  async function render(devices: Device[]): Promise<ComponentFixture<DeviceList>> {
    const store = TestBed.inject(DevicesStore);
    store.refresh();

    http.expectOne('/api/devices').flush({ devices, total: devices.length });
    http.expectOne('/api/health').flush({ status: 'ok', database: true, sources: {} });
    flushRoomOrder();

    const fixture = TestBed.createComponent(DeviceList);
    await fixture.whenStable();
    return fixture;
  }

  function rows(fixture: ComponentFixture<DeviceList>): HTMLElement[] {
    return [
      ...(fixture.nativeElement as HTMLElement).querySelectorAll<HTMLElement>('tr.device-row'),
    ];
  }

  const sample = [
    makeDevice({ id: 'io://1', name: 'Volet salon', room: 'Salon' }),
    makeDevice({ id: 'io://2', name: 'Volet cuisine', room: 'Cuisine' }),
    makeStation({ id: '70:ee:50:01', name: 'Station', room: 'Bureau' }),
  ];

  it('liste tous les équipements', async () => {
    const fixture = await render(sample);
    expect(rows(fixture).length).toBe(3);
  });

  it('affiche un message quand aucun équipement ne correspond', async () => {
    const fixture = await render([]);

    expect((fixture.nativeElement as HTMLElement).textContent).toContain(
      'Aucun équipement ne correspond',
    );
  });

  it('filtre par source', async () => {
    const fixture = await render(sample);
    const component = fixture.componentInstance as unknown as {
      sourceFilter: { set(v: string | null): void };
      filtered(): Device[];
    };

    component.sourceFilter.set('netatmo');
    await fixture.whenStable();

    expect(component.filtered().map((d) => d.name)).toEqual(['Station']);
    expect(rows(fixture).length).toBe(1);
  });

  it('filtre par pièce', async () => {
    const fixture = await render(sample);
    const component = fixture.componentInstance as unknown as {
      roomFilter: { set(v: string | null): void };
      filtered(): Device[];
    };

    component.roomFilter.set('Salon');
    await fixture.whenStable();

    expect(component.filtered().map((d) => d.name)).toEqual(['Volet salon']);
  });

  it('combine les deux filtres', async () => {
    const fixture = await render(sample);
    const component = fixture.componentInstance as unknown as {
      sourceFilter: { set(v: string | null): void };
      roomFilter: { set(v: string | null): void };
      filtered(): Device[];
    };

    // Aucun équipement Netatmo dans le salon : la combinaison doit être vide,
    // pas retomber sur l'un des deux filtres.
    component.sourceFilter.set('netatmo');
    component.roomFilter.set('Salon');
    await fixture.whenStable();

    expect(component.filtered()).toEqual([]);
  });

  it('propose les pièces réellement présentes', async () => {
    const fixture = await render(sample);
    const component = fixture.componentInstance as unknown as {
      roomOptions(): { label: string; value: string | null }[];
    };

    expect(component.roomOptions().map((o) => o.value)).toEqual([
      null,
      'Bureau',
      'Cuisine',
      'Salon',
    ]);
  });

  it('ouvre le détail au clic sur une ligne', async () => {
    const fixture = await render(sample);
    const router = TestBed.inject(Router);
    const navigate = vi.spyOn(router, 'navigate').mockResolvedValue(true);

    rows(fixture)[0].click();
    await fixture.whenStable();

    // Le tri est fait côté backend (ORDER BY room, name) ; la liste préserve
    // l'ordre reçu, donc celui du tableau fourni ici.
    expect(navigate).toHaveBeenCalledWith(['/devices', 'io://1']);
  });

  it("n'affiche des commandes que pour les équipements pilotables", async () => {
    const fixture = await render(sample);

    // Deux volets TaHoma joignables ; la station Netatmo est en lecture seule.
    const commandCells = (fixture.nativeElement as HTMLElement).querySelectorAll('.row-commands');
    expect(commandCells.length).toBe(2);
  });

  it('envoie la commande sans ouvrir le détail', async () => {
    const fixture = await render([makeDevice({ id: 'io://1' })]);
    const router = TestBed.inject(Router);
    const navigate = vi.spyOn(router, 'navigate').mockResolvedValue(true);

    const button = (fixture.nativeElement as HTMLElement).querySelector(
      '.row-commands button',
    ) as HTMLButtonElement;
    button.click();
    await fixture.whenStable();

    const req = http.expectOne(`/api/devices/${encodeURIComponent('io://1')}/command`);
    expect(req.request.body).toEqual({ command: 'open', parameters: [] });
    req.flush({ exec_id: 'exec-1' });

    // La ligne entière est cliquable : sans arrêt de la propagation, agir sur
    // un volet déclencherait aussi la navigation.
    expect(navigate).not.toHaveBeenCalled();
  });
});
