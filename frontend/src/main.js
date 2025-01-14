import {Url} from '../wailsjs/go/main/App';

// Changes window location to the Raw Panel Explorer webserver URL supplied by the Url() function
window.onload = function () {
    // Call App.Greet(name)
    try {
        Url()
            .then((result) => {
                // Update result with data back from App.Greet()
                window.location = result
            })
            .catch((err) => {
                console.error(err);
            });
    } catch (err) {
        console.error(err);
    }
};
