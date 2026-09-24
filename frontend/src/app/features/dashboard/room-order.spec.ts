import { HttpTestingController } from '@angular/common/http/testing';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { OrderList } from '@openng/optimus-ui/orderlist';

import { RoomOrder } from './room-order';
import { DevicesStore } from '../../core/devices.store';
import { flushRefresh, makeDevice, testProviders } from '../../testing/providers';

describe('RoomOrder', () => {
  let http: HttpTestingController;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [RoomOrder],
      providers: testProviders(),
    }).compileComponents();

    http = TestBed.inject(HttpTestingController);
  });

  afterEach(() => http.verify());

  async function render(): Promise<ComponentFixture<RoomOrder>> {
    TestBed.inject(DevicesStore).refresh();
    flushRefresh(
      [
        makeDevice({ id: 'a', room: 'Salon' }),
        makeDevice({ id: 'b', room: 'Cuisine' }),
        makeDevice({ id: 'c', room: 'Lily' }),
      ],
      ['Salon'],
    );

    const fixture = TestBed.createComponent(RoomOrder);
    await fixture.whenStable();

    (fixture.nativeElement as HTMLElement)
      .querySelector<HTMLButtonElement>('.open-room-order button')!
      .click();
    await fixture.whenStable();
    return fixture;
  }

  function items(): string[] {
    return [...document.querySelectorAll('.room-item')].map((el) => el.textContent?.trim() ?? '');
  }

  function click(selector: string): void {
    document.querySelector<HTMLButtonElement>(`${selector} button`)!.click();
  }

  it("liste les pièces dans l'ordre d'affichage actuel", async () => {
    await render();
    expect(items()).toEqual(['Salon', 'Cuisine', 'Lily']);
  });

  it("enregistre l'ordre modifié, toutes les pièces classées", async () => {
    const fixture = await render();

    // Faire monter « Lily » en tête, comme avec les flèches de la liste.
    const list = fixture.debugElement.query(By.directive(OrderList)).componentInstance as OrderList;
    list.selection = ['Lily'];
    list.moveTop();
    await fixture.whenStable();
    expect(items()).toEqual(['Lily', 'Salon', 'Cuisine']);

    click('.save-order');
    const req = http.expectOne('/api/rooms/order');
    expect(req.request.method).toBe('PUT');
    expect(req.request.body).toEqual({ rooms: ['Lily', 'Salon', 'Cuisine'] });
    req.flush({ rooms: ['Lily', 'Salon', 'Cuisine'] });
  });

  it("revient à l'ordre alphabétique", async () => {
    await render();

    click('.reset-order');
    const req = http.expectOne('/api/rooms/order');
    expect(req.request.body).toEqual({ rooms: [] });
    req.flush({ rooms: [] });

    expect(TestBed.inject(DevicesStore).rooms()).toEqual(['Cuisine', 'Lily', 'Salon']);
  });
});
