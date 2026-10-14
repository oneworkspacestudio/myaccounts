"""Minimal OIDC relying party. Authlib owns discovery, PKCE, state and ID-token validation."""
import os, secrets, datetime
from urllib.parse import urlparse
from flask import Flask, session, redirect, request, abort, jsonify, render_template_string
from authlib.integrations.flask_client import OAuth

issuer = os.environ['OIDC_ISSUER'].rstrip('/')
origin = os.environ.get('RP_ORIGIN', 'http://localhost:18995').rstrip('/')
site = urlparse(origin)
if site.scheme != 'https' and site.hostname not in ('localhost', '127.0.0.1'):
    raise RuntimeError('Use HTTPS for a public relying party')
app = Flask(__name__)
app.config.update(SECRET_KEY=os.environ.get('RP_SESSION_SECRET') or secrets.token_hex(32),
    SESSION_COOKIE_HTTPONLY=True, SESSION_COOKIE_SAMESITE='Lax', SESSION_COOKIE_SECURE=site.scheme == 'https',
    PERMANENT_SESSION_LIFETIME=datetime.timedelta(minutes=30), TRUSTED_HOSTS=[site.hostname])
oauth = OAuth(app)
oauth.register(name='myaccounts', client_id=os.environ['OIDC_CLIENT_ID'], client_secret=os.environ['OIDC_CLIENT_SECRET'],
    server_metadata_url=issuer+'/.well-known/openid-configuration',
    client_kwargs={'scope':'openid profile email', 'code_challenge_method':'S256'})

@app.get('/')
def home():
    return render_template_string('<h1>OIDC example</h1>{% if user %}<p>Signed in as {{ user.sub }}</p><form method="post" action="/logout"><button>Sign out</button></form>{% else %}<a href="/login">Sign in with MyAccounts</a>{% endif %}', user=session.get('user'))

@app.get('/login')
def login(): return oauth.myaccounts.authorize_redirect(origin+'/callback')

@app.get('/callback')
def callback():
    token=oauth.myaccounts.authorize_access_token()
    user=token['userinfo']  # Authlib validated the OIDC ID token, including nonce.
    if not user.get('sub'): abort(401)
    session.clear(); session.permanent=True
    session['user']={key:user.get(key) for key in ('sub','name','email')}
    return redirect('/')

@app.get('/api/me')
def me():
    if not session.get('user'): abort(401)
    return jsonify(session['user'])

@app.post('/logout')
def logout():
    if request.headers.get('Origin') != origin: abort(403)
    session.clear(); return redirect('/')

if __name__=='__main__': app.run(host='127.0.0.1', port=site.port or 18995, debug=False)
