# Evidence: xb-account

## Principals, provisioning and revocation

**Generic password verification is not Kerberos proof.** Neither a core password
check, a provisioning token, nor the stored device proof MAC authenticates a
native Kerberos exchange; xb-auth must validate pre-authentication itself.
