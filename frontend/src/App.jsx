import { Navigate, NavLink, Route, Routes } from 'react-router-dom';
import Login from './pages/Login';
import Vehicles from './pages/Vehicles';
import Deliveries from './pages/Deliveries';

function RequireAuth({ children }) {
  return localStorage.getItem('token') ? children : <Navigate to="/login" replace />;
}

// /map (web/map.html, the C8/C10 demo page) is served by the Go backend
// itself, not this SPA — different origin, different port. Linking out
// to it with the current token saved a second dev-token mint.
function mapURL() {
  const apiBase = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080/api/v1';
  const backendOrigin = apiBase.replace(/\/api\/v1\/?$/, '');
  const token = localStorage.getItem('token') || '';
  return `${backendOrigin}/map?token=${encodeURIComponent(token)}`;
}

export default function App() {
  const signedIn = !!localStorage.getItem('token');

  return (
    <div>
      {signedIn && (
        <nav>
          <NavLink to="/vehicles">Vehicles</NavLink>
          <NavLink to="/deliveries">Deliveries</NavLink>
          <a href={mapURL()} target="_blank" rel="noreferrer">
            Live map
          </a>
          <button
            className="link-button"
            onClick={() => {
              localStorage.removeItem('token');
              window.location.href = '/login';
            }}
          >
            Sign out
          </button>
        </nav>
      )}

      <main>
        <Routes>
          <Route path="/login" element={<Login />} />
          <Route
            path="/vehicles"
            element={
              <RequireAuth>
                <Vehicles />
              </RequireAuth>
            }
          />
          <Route
            path="/deliveries"
            element={
              <RequireAuth>
                <Deliveries />
              </RequireAuth>
            }
          />
          <Route path="*" element={<Navigate to={signedIn ? '/vehicles' : '/login'} replace />} />
        </Routes>
      </main>
    </div>
  );
}