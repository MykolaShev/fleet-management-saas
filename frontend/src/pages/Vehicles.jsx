import { useEffect, useState } from 'react';
import { api } from '../api';

export default function Vehicles() {
  const [vehicles, setVehicles] = useState(null);
  const [error, setError] = useState(null);

  useEffect(() => {
    api('/vehicles?page_size=200')
      .then((data) => setVehicles(data.items || []))
      .catch((err) => setError(err.message));
  }, []);

  if (error) return <p className="error">{error}</p>;
  if (!vehicles) return <p>Loading…</p>;

  return (
    <div>
      <h2>Vehicles ({vehicles.length})</h2>
      <table>
        <thead>
          <tr>
            <th>Plate</th>
            <th>Model</th>
            <th>Status</th>
            <th>Position</th>
          </tr>
        </thead>
        <tbody>
          {vehicles.map((v) => (
            <tr key={v.id}>
              <td>{v.plate_number}</td>
              <td>{v.model}</td>
              <td>{v.status}</td>
              <td>
                {v.current_lat != null
                  ? `${v.current_lat.toFixed(4)}, ${v.current_lng.toFixed(4)}`
                  : '—'}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}