import { Navigate, NavLink, Route, Routes } from 'react-router-dom';
import Login from './pages/Login';
import Vehicles from './pages/Vehicles';
import Deliveries from './pages/Deliveries';
import { useAuth } from './auth';

function RequireAuth({ children }) {
  const { signedIn } = useAuth();
  return signedIn ? children : <Navigate to="/login" replace />;
}

// /map (web/map.html, the C8/C10 demo page) is served by the Go backend
// itself, not this SPA — different origin, different port. Linking out
// with the current token saves a second dev-token mint.
function mapURL(token) {
  const apiBase = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080/api/v1';
  const backendOrigin = apiBase.replace(/\/api\/v1\/?$/, '');
  return `${backendOrigin}/map?token=${encodeURIComponent(token || '')}`;
}

function AccountInfo() {
  const { tenantId, token, signOut } = useAuth();
  return (
    <span className="account-info">
      tenant <code>{(tenantId || '').slice(0, 8)}…</code>
      <button className="link-button" onClick={() => navigator.clipboard.writeText(token || '')}>
        copy token
      </button>
      <button className="link-button" onClick={signOut}>
        sign out
      </button>
    </span>
  );
}

export default function App() {
  const { signedIn, token } = useAuth();

  return (
    <div>
      {signedIn && (
        <header className="dispatch-bar">
          <span className="brand">FLEET</span>
          <nav>
            <NavLink to="/vehicles">Vehicles</NavLink>
            <NavLink to="/deliveries">Deliveries</NavLink>
            <a href={mapURL(token)} target="_blank" rel="noreferrer">
              Live map ↗
            </a>
          </nav>
          <AccountInfo />
        </header>
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