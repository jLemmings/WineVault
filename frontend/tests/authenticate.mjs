// Live suites use an existing test owner; they never claim an unconfigured installation.
export const authHeaders={'X-WineVault-Request':'1'}
export async function authenticate(page,base){
 const username=process.env.WINEVAULT_TEST_USERNAME,password=process.env.WINEVAULT_TEST_PASSWORD
 if(!username||!password)throw new Error('Set WINEVAULT_TEST_USERNAME and WINEVAULT_TEST_PASSWORD for an existing owner before running live browser tests.')
 const response=await page.request.post(`${base}/api/auth/login`,{data:{username,password},headers:authHeaders})
 if(!response.ok())throw new Error(`Test owner login failed (${response.status()}). Check credentials and complete owner setup first.`)
}
