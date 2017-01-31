# Versioning the Portlayer API

Versioning the API is a requirement when exposing it out of process. As such the mechanism by which it is versioned is
an implementation property of the mechanism by which it is exposed.

For session based mechanisms, a simple approach would be:
* during initial session establishment, the client specifies the version it requires
* the server associates a translation chain with the session
  * this could be for the entire API surface, or just individual calls
  * if some calls in the old API cannot be translated these should be reported as unavailable

* when the client makes a call, the server:
  * checks if translation is required
  * translation is performed
  * issues in translation (e.g. limited ability to translate an argument) should be returned to the client in
    the same fashion as other input validation errors, with an error message noting is a version compatibility issue,
    and whether that argument data is unsupported, that argument is unsupported or that call is unsupported.
  * if the call is made the result should pass back through the translation chain
    * applies irrespective of error or success

