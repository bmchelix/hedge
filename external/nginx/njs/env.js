function getAuth(r) {
    let external_auth = process.env["IS_EXTERNAL_AUTH"];
    return external_auth !== undefined ? external_auth : "false";
}

export default {getAuth}