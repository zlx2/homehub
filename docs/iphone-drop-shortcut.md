# iPhone quick upload to Drop

HomeHub supports a revocable device token whose only permission is creating a
new Drop item. It cannot list, read, download, update, or delete Drop content,
and cannot access another HomeHub service.

## Create the token

1. Sign in to HomeHub as the owner.
2. Open **分享与设备访问 → 快捷分享令牌**.
3. Keep the name `iPhone 快捷分享`, choose an expiry, and create the token.
4. Copy the `hht_...` value immediately. HomeHub stores only its hash and will
   not display the raw value again.
5. Save a recovery copy in Bitwarden, then paste it into the shortcut below.

## Build the shortcut

Create a shortcut named `上传到 Drop` and enable **Show in Share Sheet**. Limit
accepted input to images and files.

Add these actions in order:

1. **Generate UUID**.
2. **Get Contents of URL**:
   - URL: `https://zlx2.com/drop/v1/items`
   - Method: `POST`
   - Header `Authorization`: `Bearer hht_your_token_here`
   - Header `Idempotency-Key`: the generated UUID
   - Request Body: `Form`
   - Form field `files`: `Shortcut Input`
   - Form field `ttl_days`: `1`
3. **Show Notification** with `已上传到 Drop`.

The first run asks for permission to contact the server. Choose **Always
Allow**. A screenshot can then be uploaded directly from its preview or from
Photos using **Share → 上传到 Drop**.

If the shortcut is lost or the phone is replaced, revoke its token in HomeHub
and create a new one.
