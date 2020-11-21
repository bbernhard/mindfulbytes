function randomInt(min, max) {
    return Math.floor(Math.random() * (max - min + 1) + min);
}

function getDataForDate(api, date) {
    api.getDataForDate("imgreader", date)
        .then(function(data) {
            return data;
        })
        .catch(function() {
            console.log("exception");
        });
}

function displayData(api, data, date) {
    var imageUrl = api.getImageUrlFromId(data.plugin, data.uuid);

	var myImage = new Image();
	myImage.onload = function() {
		$("#img").height(this.height);
		$("#img").width(this.width);
		$("#img").attr("src", this.src);
	}
	//myImage.onerror = loadFailure;
	myImage.src = imageUrl;

    $("#when").text(moment(date).fromNow() + " this picture was taken");
    $("#whenDetailed").text(date);
}

$(document).ready(function() {
    document.title = "MindfulBytes";

    let baseUrl = document.getElementById("imageReaderScriptFile").getAttribute("data-base-url");
    currentDate = moment().format("MM-DD");
    api = new MindfulBytesApi(baseUrl);


    api.getDataForDate("imgreader", currentDate)
        .then(function(data) {
            if (data.response.length == 0) { //nothing found, pick a random image
                api.getFullDates("imgreader")
                    .then(function(data) {
                        if (data.length > 0) {
                            var randomDate = data[randomInt(0, data.length - 1)];
                            api.getDataForFullDate("imgreader", randomDate)
                                .then(function(dataForDate) {
                                    let randomPos = randomInt(0, dataForDate.response.length - 1);
                                    displayData(api, dataForDate.response[randomPos], randomDate);
                                })

                                .catch(function(e) {
                                    console.error(e.stack);
                                });
                        }
                    })
                    .catch(function(e) {
                        console.error(e.stack);
                    });
            } else { //found something for today
                displayData(api, data, moment().format("YYYY-MM-DD"));
            }
        })

        .catch(function(e) {
            console.log(e.stack);
        });
});
