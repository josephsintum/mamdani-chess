//! The `guest` cookie: who the caller is. There are no accounts; a browser
//! gets a random ID the first time it calls the API and keeps it for a year.

use std::convert::Infallible;

use axum::extract::FromRequestParts;
use axum::http::request::Parts;
use axum_extra::extract::CookieJar;
use axum_extra::extract::cookie::{Cookie, SameSite};

use crate::game::GuestId;

const COOKIE: &str = "guest";
const ONE_YEAR: cookie::time::Duration = cookie::time::Duration::days(365);

/// The caller's guest ID. `jar` carries a new cookie when the caller didn't
/// have a valid one, so handlers must return it in their response.
pub struct Guest {
    pub id: GuestId,
    pub jar: CookieJar,
}

impl<S: Send + Sync> FromRequestParts<S> for Guest {
    type Rejection = Infallible;

    async fn from_request_parts(parts: &mut Parts, _: &S) -> Result<Guest, Infallible> {
        let jar = CookieJar::from_headers(&parts.headers);
        if let Some(id) = jar.get(COOKIE).and_then(|c| c.value().parse().ok()) {
            return Ok(Guest { id, jar });
        }
        let id = GuestId::random();
        let https = parts
            .headers
            .get("x-forwarded-proto")
            .is_some_and(|v| v.as_bytes() == b"https");
        let cookie = Cookie::build((COOKIE, id.to_string()))
            .path("/")
            .max_age(ONE_YEAR)
            .http_only(true)
            .same_site(SameSite::Lax)
            .secure(https);
        Ok(Guest {
            id,
            jar: jar.add(cookie),
        })
    }
}
