package app

import (
	"context"
	"encoding/json"
	"sync"
)

type Store interface {
	Delete(context.Context, Family) error
	GetUser(context.Context, string) (User, error)
	Users(context.Context) ([]User, error)
	SaveUser(context.Context, User, int) error
	Get(context.Context, string) (Family, error)
	List(context.Context) ([]Family, error)
	Create(context.Context, Family) error
	Save(context.Context, Family, int) error
	PutDevice(context.Context, Device) error
	GetDevice(context.Context, string) (Device, error)
	Devices(context.Context) ([]Device, error)
}
type MemoryStore struct {
	mu       sync.Mutex
	families map[string]Family
	devices  map[string]Device
	users    map[string]User
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{families: map[string]Family{}, devices: map[string]Device{}, users: map[string]User{}}
}
func clone(f Family) Family {
	b, _ := json.Marshal(f)
	var out Family
	_ = json.Unmarshal(b, &out)
	return out
}
func (m *MemoryStore) Get(_ context.Context, id string) (Family, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, ok := m.families[id]
	if !ok {
		return f, ErrNotFound
	}
	return clone(f), nil
}
func (m *MemoryStore) List(_ context.Context) ([]Family, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []Family{}
	for _, f := range m.families {
		out = append(out, clone(f))
	}
	return out, nil
}
func (m *MemoryStore) Create(_ context.Context, f Family) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.families[f.ID]; ok {
		return ErrConflict
	}
	phone, err := NormalizePhone(f.Phone)
	if err != nil {
		return err
	}
	for _, old := range m.families {
		existing, _ := NormalizePhone(old.Phone)
		if existing == phone {
			return ErrConflict
		}
	}
	f.Phone = phone
	m.families[f.ID] = clone(f)
	return nil
}
func (m *MemoryStore) Save(_ context.Context, f Family, rev int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	old, ok := m.families[f.ID]
	if !ok {
		return ErrNotFound
	}
	if old.Revision != rev {
		return ErrConflict
	}
	m.families[f.ID] = clone(f)
	return nil
}
func (m *MemoryStore) PutDevice(_ context.Context, d Device) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.devices[d.ID] = d
	return nil
}
func (m *MemoryStore) GetDevice(_ context.Context, id string) (Device, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.devices[id]
	if !ok {
		return d, ErrNotFound
	}
	return d, nil
}
func (m *MemoryStore) Devices(_ context.Context) ([]Device, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ds := []Device{}
	for _, d := range m.devices {
		ds = append(ds, d)
	}
	return ds, nil
}

func (m *MemoryStore) GetUser(_ context.Context, id string) (User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[id]
	if !ok {
		return u, ErrNotFound
	}
	return u, nil
}
func (m *MemoryStore) Users(_ context.Context) ([]User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []User{}
	for _, u := range m.users {
		out = append(out, u)
	}
	return out, nil
}
func (m *MemoryStore) SaveUser(_ context.Context, u User, rev int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	old, ok := m.users[u.ID]
	if (rev == 0 && ok) || (rev != 0 && (!ok || old.Revision != rev)) {
		return ErrConflict
	}
	m.users[u.ID] = u
	return nil
}

func (m *MemoryStore) Delete(_ context.Context, f Family) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	old, ok := m.families[f.ID]
	if !ok {
		return ErrNotFound
	}
	if old.Revision != f.Revision {
		return ErrConflict
	}
	delete(m.families, f.ID)
	return nil
}
