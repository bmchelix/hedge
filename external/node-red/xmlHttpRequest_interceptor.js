(function(originalXhrSend) {
    // Function to get a cookie's value by its name
    function getCookie(name) {
        let cookieString = document.cookie;
        let cookieArray = cookieString.split(';');
        for (let i = 0; i < cookieArray.length; i++) {
            let cookie = cookieArray[i].trim();
            let cookieParts = cookie.split('=');
            if (cookieParts[0] === name) {
                return cookieParts[1];
            }
        }
        console.log('csrf_token not found');
        return "";
    }

    // Modify the XMLHttpRequest send function
    XMLHttpRequest.prototype.send = function(data) {
        // Retrieve the CSRF token from cookies
        const csrfToken = getCookie('csrf_token');

        // Set the CSRF token as a header if it exists
        if (csrfToken) {
            this.setRequestHeader('X-CSRF-Token', csrfToken);
        } else {
            console.log('No CSRF token available to set as header.');
        }

        // Call the original send method
        originalXhrSend.call(this, data);
    };
})(XMLHttpRequest.prototype.send);