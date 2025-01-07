import {Url} from '../wailsjs/go/main/App';

// Setup the greet function
window.onload = function () {
    // Access the iframe element
    const iframe = document.getElementById('iframe');

    // Call App.Greet(name)
    try {
        Url()
            .then((result) => {
                // Update result with data back from App.Greet()
                //iframe.src = result;
                window.location = result
            })
            .catch((err) => {
                console.error(err);
            });
    } catch (err) {
        console.error(err);
    }
};
