import { Alert, Button, Card, Heading, Input, Page, Section, Stack, Text } from "./oneaccount-ui"

export function OneAccountSecurityScreen({ data, onAction }: { data: any; onAction: (name: string) => void }) {
  return (
    <Page>
      <Stack>
        <Heading level={1}>Security</Heading>
        <Text>Two-factor authentication with One Authenticator</Text>
        {data.message && (
          <Alert role="status">{data.message}</Alert>
        )}
        {data.error && (
          <Alert role="alert">{data.error}</Alert>
        )}
        {data.recovery && (
          <Card>
            <Heading level={2}>Save your ten recovery codes</Heading>
            <Text>These codes are shown once. Each works only once. Save them separately from your phone, never in My Drive.</Text>
            <Section variant="recovery" value={data.recovery} />
            <Button onClick={() => onAction("download")}>Download recovery codes</Button>
            <Button onClick={() => onAction("print")}>Print recovery codes</Button>
            <Button onClick={() => onAction("acknowledge")}>I have saved my recovery codes</Button>
          </Card>
        )}
        {data.showStatus && (
          <Card>
            <Heading level={2}>{data.statusTitle}</Heading>
            <Text>{data.metadata}</Text>
            <Text>Generate your code locally in One Authenticator. The phone app needs no OneAccount session or internet after setup.</Text>
            {data.enabled && (
              <Text>Replace a lost device by signing in with a recovery code, disabling the old authenticator, then enrolling the replacement. Old seeds are never cloned. A recent MFA sign-in can authorize security changes for five minutes after you verify your password. Disabling signs out all sessions.</Text>
            )}
            <Input name="password" label="Current OneAccount password" type="password" autocomplete="current-password" />
            {data.enabled && (
              <>
                <Input name="proof" label="Current code or recovery code (optional immediately after MFA sign-in)" autocomplete="off" />
                <Button onClick={() => onAction("regenerate")}>Regenerate recovery codes</Button>
                <Input name="disableConfirmation" label="Type DISABLE to remove the authenticator" autocomplete="off" />
                <Button variant="danger" onClick={() => onAction("disable")}>Disable two-factor authentication</Button>
              </>
            )}
            {data.disabled && (
              <>
                <Input name="deviceLabel" label="Device label" autocomplete="off" />
                <Button onClick={() => onAction("begin")}>Enable two-factor authentication</Button>
              </>
            )}
            <Text>TOTP is vulnerable to phishing. Verify your OneAccount address before entering a code.</Text>
            <Heading level={2}>Recent security notifications</Heading>
            <Text>{data.notifications}</Text>
          </Card>
        )}
        {data.pending && (
          <Card>
            <Heading level={2}>Scan with One Authenticator</Heading>
            <Text>Open One Authenticator on your phone → Add OneAccount → Scan QR code. Keep this QR private. Setup expires in five minutes.</Text>
            <Section variant="qr" value={data.qr} />
            <Section variant="key" value={data.secret} />
            <Input name="code" label="Six-digit code from One Authenticator" autocomplete="one-time-code" />
            <Button onClick={() => onAction("confirm")}>Verify and enable</Button>
            <Button variant="secondary" onClick={() => onAction("cancel")}>Cancel setup</Button>
          </Card>
        )}
      </Stack>
    </Page>
  )
}
