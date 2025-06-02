import { Url } from '../wailsjs/go/main/App';
import { OpenURLInBrowser } from '../wailsjs/go/main/App';

window.addEventListener("message", (event) => {
    if (event.data?.type === "open-external" && event.data.url) {
        OpenURLInBrowser(event.data.url)
            .catch(err => console.error("Failed to open URL in browser:", err));
    }
});

// Changes window location to the Raw Panel Explorer webserver URL supplied by the Url() function
window.onload = function () {
    // try {
    //     Url()
    //         .then((result) => {
    //             // Update result with data back from App.Greet()
    //             window.location = result

    //         })
    //         .catch((err) => {
    //             console.error(err);
    //         });
    // } catch (err) {
    //     console.error(err);
    // }

    const iframe = document.getElementById("explorer");

    Url()
        .then((result) => {
            console.log("Explorer URL from Go:", result);
            iframe.src = result;
        })
        .catch((err) => {
            console.error("Failed to get URL from backend:", err);
        });
};
