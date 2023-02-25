package common

import (
	"strings"
)

// Oh boy.
var Santizier = strings.NewReplacer(
	"\u200B", "",
	"\u200C", "",
	"\u200D", "",
	"\uFEFF", "",
	" ", " ",
	" ", " ",
	" ", " ",
	" ", " ",
	" ", " ",
	" ", " ",
	" ", " ",
	" ", " ",
	" ", " ",
	" ", " ",
	" ", " ",
	" ", " ",
	" ", " ",
	" ", " ",
	" ", " ",
	"⍺", "a",
	"ａ", "a",
	"𝐚", "a",
	"𝑎", "a",
	"𝒂", "a",
	"𝒶", "a",
	"𝓪", "a",
	"𝔞", "a",
	"𝕒", "a",
	"𝖆", "a",
	"𝖺", "a",
	"𝗮", "a",
	"𝘢", "a",
	"𝙖", "a",
	"𝚊", "a",
	"ɑ", "a",
	"𝛂", "a",
	"𝛼", "a",
	"𝜶", "a",
	"𝝰", "a",
	"𝞪", "a",
	"𝐛", "b",
	"𝑏", "b",
	"𝒃", "b",
	"𝒷", "b",
	"𝓫", "b",
	"𝔟", "b",
	"𝕓", "b",
	"𝖇", "b",
	"𝖻", "b",
	"𝗯", "b",
	"𝘣", "b",
	"𝙗", "b",
	"𝚋", "b",
	"ｃ", "c",
	"ⅽ", "c",
	"𝐜", "c",
	"𝑐", "c",
	"𝒄", "c",
	"𝒸", "c",
	"𝓬", "c",
	"𝔠", "c",
	"𝕔", "c",
	"𝖈", "c",
	"𝖼", "c",
	"𝗰", "c",
	"𝘤", "c",
	"𝙘", "c",
	"𝚌", "c",
	"ᴄ", "c",
	"ⅾ", "d",
	"ⅆ", "d",
	"𝐝", "d",
	"𝑑", "d",
	"𝒅", "d",
	"𝒹", "d",
	"𝓭", "d",
	"𝔡", "d",
	"𝕕", "d",
	"𝖉", "d",
	"𝖽", "d",
	"𝗱", "d",
	"𝘥", "d",
	"𝙙", "d",
	"𝚍", "d",
	"℮", "e",
	"ｅ", "e",
	"ℯ", "e",
	"ⅇ", "e",
	"𝐞", "e",
	"𝑒", "e",
	"𝒆", "e",
	"𝓮", "e",
	"𝔢", "e",
	"𝕖", "e",
	"𝖊", "e",
	"𝖾", "e",
	"𝗲", "e",
	"𝘦", "e",
	"𝙚", "e",
	"𝚎", "e",
	"𝐟", "f",
	"𝑓", "f",
	"𝒇", "f",
	"𝒻", "f",
	"𝓯", "f",
	"𝔣", "f",
	"𝕗", "f",
	"𝖋", "f",
	"𝖿", "f",
	"𝗳", "f",
	"𝘧", "f",
	"𝙛", "f",
	"𝚏", "f",
	"ẝ", "f",
	"ｇ", "g",
	"ℊ", "g",
	"𝐠", "g",
	"𝑔", "g",
	"𝒈", "g",
	"𝓰", "g",
	"𝔤", "g",
	"𝕘", "g",
	"𝖌", "g",
	"𝗀", "g",
	"𝗴", "g",
	"𝘨", "g",
	"𝙜", "g",
	"𝚐", "g",
	"ɡ", "g",
	"ᶃ", "g",
	"ƍ", "g",
	"ｈ", "h",
	"ℎ", "h",
	"𝐡", "h",
	"𝒉", "h",
	"𝒽", "h",
	"𝓱", "h",
	"𝔥", "h",
	"𝕙", "h",
	"𝖍", "h",
	"𝗁", "h",
	"𝗵", "h",
	"𝘩", "h",
	"𝙝", "h",
	"𝚑", "h",
	"˛", "i",
	"⍳", "i",
	"ｉ", "i",
	"ⅰ", "i",
	"ℹ", "i",
	"ⅈ", "i",
	"𝐢", "i",
	"𝑖", "i",
	"𝒊", "i",
	"𝒾", "i",
	"𝓲", "i",
	"𝔦", "i",
	"𝕚", "i",
	"𝖎", "i",
	"𝗂", "i",
	"𝗶", "i",
	"𝘪", "i",
	"𝙞", "i",
	"𝚒", "i",
	"ı", "i",
	"𝚤", "i",
	"ɪ", "i",
	"ɩ", "i",
	"𝛊", "i",
	"𝜄", "i",
	"𝜾", "i",
	"𝝸", "i",
	"𝞲", "i",
	"ｊ", "j",
	"ⅉ", "j",
	"𝐣", "j",
	"𝑗", "j",
	"𝒋", "j",
	"𝒿", "j",
	"𝓳", "j",
	"𝔧", "j",
	"𝕛", "j",
	"𝖏", "j",
	"𝗃", "j",
	"𝗷", "j",
	"𝘫", "j",
	"𝙟", "j",
	"𝚓", "j",
	"𝐤", "k",
	"𝑘", "k",
	"𝒌", "k",
	"𝓀", "k",
	"𝓴", "k",
	"𝔨", "k",
	"𝕜", "k",
	"𝖐", "k",
	"𝗄", "k",
	"𝗸", "k",
	"𝘬", "k",
	"𝙠", "k",
	"𝚔", "k",
	"ᴋ", "k",
	"ĸ", "k",
	"𝛋", "k",
	"𝛞", "k",
	"𝜅", "k",
	"𝜘", "k",
	"𝜿", "k",
	"𝝒", "k",
	"𝝹", "k",
	"𝞌", "k",
	"𝞳", "k",
	"𝟆", "k",
	"|", "l",
	"∣", "l",
	"￨", "l",
	"1", "l",
	"𝟏", "l",
	"𝟙", "l",
	"𝟣", "l",
	"𝟭", "l",
	"𝟷", "l",
	"ℐ", "l",
	"ℑ", "l",
	"𝐈", "l",
	"𝐼", "l",
	"𝑰", "l",
	"𝓘", "l",
	"𝕀", "l",
	"𝕴", "l",
	"𝖨", "l",
	"𝗜", "l",
	"𝘐", "l",
	"𝙄", "l",
	"𝙸", "l",
	"ｌ", "l",
	"ⅼ", "l",
	"ℓ", "l",
	"𝐥", "l",
	"𝑙", "l",
	"𝒍", "l",
	"𝓁", "l",
	"𝓵", "l",
	"𝔩", "l",
	"𝕝", "l",
	"𝖑", "l",
	"𝗅", "l",
	"𝗹", "l",
	"𝘭", "l",
	"𝙡", "l",
	"𝚕", "l",
	"ǀ", "l",
	"𝚰", "l",
	"𝛪", "l",
	"𝜤", "l",
	"𝝞", "l",
	"𝞘", "l",
	"𝐧", "n",
	"𝑛", "n",
	"𝒏", "n",
	"𝓃", "n",
	"𝓷", "n",
	"𝔫", "n",
	"𝕟", "n",
	"𝖓", "n",
	"𝗇", "n",
	"𝗻", "n",
	"𝘯", "n",
	"𝙣", "n",
	"𝚗", "n",
	"ℼ", "n",
	"𝛑", "n",
	"𝛡", "n",
	"𝜋", "n",
	"𝜛", "n",
	"𝝅", "n",
	"𝝕", "n",
	"𝝿", "n",
	"𝞏", "n",
	"𝞹", "n",
	"𝟉", "n",
	"‎٥‎", "o",
	"ｏ", "o",
	"ℴ", "o",
	"𝐨", "o",
	"𝑜", "o",
	"𝒐", "o",
	"𝓸", "o",
	"𝔬", "o",
	"𝕠", "o",
	"𝖔", "o",
	"𝗈", "o",
	"𝗼", "o",
	"𝘰", "o",
	"𝙤", "o",
	"𝚘", "o",
	"ᴏ", "o",
	"ᴑ", "o",
	"𝛐", "o",
	"𝜊", "o",
	"𝝄", "o",
	"𝝾", "o",
	"𝞸", "o",
	"𝛔", "o",
	"𝜎", "o",
	"𝝈", "o",
	"𝞂", "o",
	"𝞼", "o",
	"⍴", "p",
	"ｐ", "p",
	"𝐩", "p",
	"𝑝", "p",
	"𝒑", "p",
	"𝓅", "p",
	"𝓹", "p",
	"𝔭", "p",
	"𝕡", "p",
	"𝖕", "p",
	"𝗉", "p",
	"𝗽", "p",
	"𝘱", "p",
	"𝙥", "p",
	"𝚙", "p",
	"𝛒", "p",
	"𝛠", "p",
	"𝜌", "p",
	"𝜚", "p",
	"𝝆", "p",
	"𝝔", "p",
	"𝞀", "p",
	"𝞎", "p",
	"𝞺", "p",
	"𝟈", "p",
	"𝐪", "q",
	"𝑞", "q",
	"𝒒", "q",
	"𝓆", "q",
	"𝓺", "q",
	"𝔮", "q",
	"𝕢", "q",
	"𝖖", "q",
	"𝗊", "q",
	"𝗾", "q",
	"𝘲", "q",
	"𝙦", "q",
	"𝚚", "q",
	"𝐫", "r",
	"𝑟", "r",
	"𝒓", "r",
	"𝓇", "r",
	"𝓻", "r",
	"𝔯", "r",
	"𝕣", "r",
	"𝖗", "r",
	"𝗋", "r",
	"𝗿", "r",
	"𝘳", "r",
	"𝙧", "r",
	"𝚛", "r",
	"ｓ", "s",
	"𝐬", "s",
	"𝑠", "s",
	"𝒔", "s",
	"𝓈", "s",
	"𝓼", "s",
	"𝔰", "s",
	"𝕤", "s",
	"𝖘", "s",
	"𝗌", "s",
	"𝘀", "s",
	"𝘴", "s",
	"𝙨", "s",
	"𝚜", "s",
	"ꜱ", "s",
	"ƽ", "s",
	"𝐭", "t",
	"𝑡", "t",
	"𝒕", "t",
	"𝓉", "t",
	"𝓽", "t",
	"𝔱", "t",
	"𝕥", "t",
	"𝖙", "t",
	"𝗍", "t",
	"𝘁", "t",
	"𝘵", "t",
	"𝙩", "t",
	"𝚝", "t",
	"ᴛ", "t",
	"𝛕", "t",
	"𝜏", "t",
	"𝝉", "t",
	"𝞃", "t",
	"𝞽", "t",
	"𝐮", "u",
	"𝑢", "u",
	"𝒖", "u",
	"𝓊", "u",
	"𝓾", "u",
	"𝔲", "u",
	"𝕦", "u",
	"𝖚", "u",
	"𝗎", "u",
	"𝘂", "u",
	"𝘶", "u",
	"𝙪", "u",
	"𝚞", "u",
	"ᴜ", "u",
	"ʋ", "u",
	"𝛖", "u",
	"𝜐", "u",
	"𝝊", "u",
	"𝞄", "u",
	"𝞾", "u",
	"∨", "v",
	"⋁", "v",
	"ｖ", "v",
	"ⅴ", "v",
	"𝐯", "v",
	"𝑣", "v",
	"𝒗", "v",
	"𝓋", "v",
	"𝓿", "v",
	"𝔳", "v",
	"𝕧", "v",
	"𝖛", "v",
	"𝗏", "v",
	"𝘃", "v",
	"𝘷", "v",
	"𝙫", "v",
	"𝚟", "v",
	"ᴠ", "v",
	"𝛎", "v",
	"𝜈", "v",
	"𝝂", "v",
	"𝝼", "v",
	"𝞶", "v",
	"×", "x",
	"╳", "x",
	"⤫", "x",
	"⤬", "x",
	"⨯", "x",
	"ｘ", "x",
	"ⅹ", "x",
	"𝐱", "x",
	"𝑥", "x",
	"𝒙", "x",
	"𝓍", "x",
	"𝔁", "x",
	"𝔵", "x",
	"𝕩", "x",
	"𝖝", "x",
	"𝗑", "x",
	"𝘅", "x",
	"𝘹", "x",
	"𝙭", "x",
	"𝚡", "x",
	"ᶌ", "y",
	"ｙ", "y",
	"𝐲", "y",
	"𝑦", "y",
	"𝒚", "y",
	"𝓎", "y",
	"𝔂", "y",
	"𝔶", "y",
	"𝕪", "y",
	"𝖞", "y",
	"𝗒", "y",
	"𝘆", "y",
	"𝘺", "y",
	"𝙮", "y",
	"𝚢", "y",
	"ʏ", "y",
	"ỿ", "y",
	"ℽ", "y",
	"𝛄", "y",
	"𝛾", "y",
	"𝜸", "y",
	"𝝲", "y",
	"𝞬", "y",
	"𝐳", "z",
	"𝑧", "z",
	"𝒛", "z",
	"𝓏", "z",
	"𝔃", "z",
	"𝔷", "z",
	"𝕫", "z",
	"𝖟", "z",
	"𝗓", "z",
	"𝘇", "z",
	"𝘻", "z",
	"𝙯", "z",
	"𝚣", "z",
	"ᴢ", "z",
	" ", " ",
	" ", " ",
	" ", " ",
	" ", " ",
	" ", " ",
	" ", " ",
	" ", " ",
	" ", " ",
	" ", " ",
	" ", " ",
	" ", " ",
	" ", " ",
	" ", " ",
	" ", " ",
	" ", " ",
	"⍺", "a",
	"ａ", "a",
	"𝐚", "a",
	"𝑎", "a",
	"𝒂", "a",
	"𝒶", "a",
	"𝓪", "a",
	"𝔞", "a",
	"𝕒", "a",
	"𝖆", "a",
	"𝖺", "a",
	"𝗮", "a",
	"𝘢", "a",
	"𝙖", "a",
	"𝚊", "a",
	"ɑ", "a",
	"𝛂", "a",
	"𝛼", "a",
	"𝜶", "a",
	"𝝰", "a",
	"𝞪", "a",
	"Ａ", "A",
	"𝐀", "A",
	"𝐴", "A",
	"𝑨", "A",
	"𝒜", "A",
	"𝓐", "A",
	"𝔄", "A",
	"𝔸", "A",
	"𝕬", "A",
	"𝖠", "A",
	"𝗔", "A",
	"𝘈", "A",
	"𝘼", "A",
	"𝙰", "A",
	"𝚨", "A",
	"𝛢", "A",
	"𝜜", "A",
	"𝝖", "A",
	"𝞐", "A",
	"𝐛", "b",
	"𝑏", "b",
	"𝒃", "b",
	"𝒷", "b",
	"𝓫", "b",
	"𝔟", "b",
	"𝕓", "b",
	"𝖇", "b",
	"𝖻", "b",
	"𝗯", "b",
	"𝘣", "b",
	"𝙗", "b",
	"𝚋", "b",
	"Ƅ", "b",
	"Ｂ", "B",
	"ℬ", "B",
	"𝐁", "B",
	"𝐵", "B",
	"𝑩", "B",
	"𝓑", "B",
	"𝔅", "B",
	"𝔹", "B",
	"𝕭", "B",
	"𝖡", "B",
	"𝗕", "B",
	"𝘉", "B",
	"𝘽", "B",
	"𝙱", "B",
	"𝚩", "B",
	"𝛣", "B",
	"𝜝", "B",
	"𝝗", "B",
	"𝞑", "B",
	"ｃ", "c",
	"ⅽ", "c",
	"𝐜", "c",
	"𝑐", "c",
	"𝒄", "c",
	"𝒸", "c",
	"𝓬", "c",
	"𝔠", "c",
	"𝕔", "c",
	"𝖈", "c",
	"𝖼", "c",
	"𝗰", "c",
	"𝘤", "c",
	"𝙘", "c",
	"𝚌", "c",
	"ᴄ", "c",
	"Ｃ", "C",
	"Ⅽ", "C",
	"ℂ", "C",
	"ℭ", "C",
	"𝐂", "C",
	"𝐶", "C",
	"𝑪", "C",
	"𝒞", "C",
	"𝓒", "C",
	"𝕮", "C",
	"𝖢", "C",
	"𝗖", "C",
	"𝘊", "C",
	"𝘾", "C",
	"𝙲", "C",
	"ⅾ", "d",
	"ⅆ", "d",
	"𝐝", "d",
	"𝑑", "d",
	"𝒅", "d",
	"𝒹", "d",
	"𝓭", "d",
	"𝔡", "d",
	"𝕕", "d",
	"𝖉", "d",
	"𝖽", "d",
	"𝗱", "d",
	"𝘥", "d",
	"𝙙", "d",
	"𝚍", "d",
	"Ⅾ", "D",
	"ⅅ", "D",
	"𝐃", "D",
	"𝐷", "D",
	"𝑫", "D",
	"𝒟", "D",
	"𝓓", "D",
	"𝔇", "D",
	"𝔻", "D",
	"𝕯", "D",
	"𝖣", "D",
	"𝗗", "D",
	"𝘋", "D",
	"𝘿", "D",
	"𝙳", "D",
	"℮", "e",
	"ｅ", "e",
	"ℯ", "e",
	"ⅇ", "e",
	"𝐞", "e",
	"𝑒", "e",
	"𝒆", "e",
	"𝓮", "e",
	"𝔢", "e",
	"𝕖", "e",
	"𝖊", "e",
	"𝖾", "e",
	"𝗲", "e",
	"𝘦", "e",
	"𝙚", "e",
	"𝚎", "e",
	"⋿", "E",
	"Ｅ", "E",
	"ℰ", "E",
	"𝐄", "E",
	"𝐸", "E",
	"𝑬", "E",
	"𝓔", "E",
	"𝔈", "E",
	"𝔼", "E",
	"𝕰", "E",
	"𝖤", "E",
	"𝗘", "E",
	"𝘌", "E",
	"𝙀", "E",
	"𝙴", "E",
	"𝚬", "E",
	"𝛦", "E",
	"𝜠", "E",
	"𝝚", "E",
	"𝞔", "E",
	"𝐟", "f",
	"𝑓", "f",
	"𝒇", "f",
	"𝒻", "f",
	"𝓯", "f",
	"𝔣", "f",
	"𝕗", "f",
	"𝖋", "f",
	"𝖿", "f",
	"𝗳", "f",
	"𝘧", "f",
	"𝙛", "f",
	"𝚏", "f",
	"ſ", "f",
	"ẝ", "f",
	"ℱ", "F",
	"𝐅", "F",
	"𝐹", "F",
	"𝑭", "F",
	"𝓕", "F",
	"𝔉", "F",
	"𝔽", "F",
	"𝕱", "F",
	"𝖥", "F",
	"𝗙", "F",
	"𝘍", "F",
	"𝙁", "F",
	"𝙵", "F",
	"𝟊", "F",
	"ｇ", "g",
	"ℊ", "g",
	"𝐠", "g",
	"𝑔", "g",
	"𝒈", "g",
	"𝓰", "g",
	"𝔤", "g",
	"𝕘", "g",
	"𝖌", "g",
	"𝗀", "g",
	"𝗴", "g",
	"𝘨", "g",
	"𝙜", "g",
	"𝚐", "g",
	"ɡ", "g",
	"ᶃ", "g",
	"ƍ", "g",
	"𝐆", "G",
	"𝐺", "G",
	"𝑮", "G",
	"𝒢", "G",
	"𝓖", "G",
	"𝔊", "G",
	"𝔾", "G",
	"𝕲", "G",
	"𝖦", "G",
	"𝗚", "G",
	"𝘎", "G",
	"𝙂", "G",
	"𝙶", "G",
	"ｈ", "h",
	"ℎ", "h",
	"𝐡", "h",
	"𝒉", "h",
	"𝒽", "h",
	"𝓱", "h",
	"𝔥", "h",
	"𝕙", "h",
	"𝖍", "h",
	"𝗁", "h",
	"𝗵", "h",
	"𝘩", "h",
	"𝙝", "h",
	"𝚑", "h",
	"Ｈ", "H",
	"ℋ", "H",
	"ℌ", "H",
	"ℍ", "H",
	"𝐇", "H",
	"𝐻", "H",
	"𝑯", "H",
	"𝓗", "H",
	"𝕳", "H",
	"𝖧", "H",
	"𝗛", "H",
	"𝘏", "H",
	"𝙃", "H",
	"𝙷", "H",
	"𝚮", "H",
	"𝛨", "H",
	"𝜢", "H",
	"𝝜", "H",
	"𝞖", "H",
	"˛", "i",
	"⍳", "i",
	"ｉ", "i",
	"ⅰ", "i",
	"ℹ", "i",
	"ⅈ", "i",
	"𝐢", "i",
	"𝑖", "i",
	"𝒊", "i",
	"𝒾", "i",
	"𝓲", "i",
	"𝔦", "i",
	"𝕚", "i",
	"𝖎", "i",
	"𝗂", "i",
	"𝗶", "i",
	"𝘪", "i",
	"𝙞", "i",
	"𝚒", "i",
	"ı", "i",
	"𝚤", "i",
	"ɪ", "i",
	"ɩ", "i",
	"𝛊", "i",
	"𝜄", "i",
	"𝜾", "i",
	"𝝸", "i",
	"𝞲", "i",
	"ｊ", "j",
	"ⅉ", "j",
	"𝐣", "j",
	"𝑗", "j",
	"𝒋", "j",
	"𝒿", "j",
	"𝓳", "j",
	"𝔧", "j",
	"𝕛", "j",
	"𝖏", "j",
	"𝗃", "j",
	"𝗷", "j",
	"𝘫", "j",
	"𝙟", "j",
	"𝚓", "j",
	"Ｊ", "J",
	"𝐉", "J",
	"𝐽", "J",
	"𝑱", "J",
	"𝒥", "J",
	"𝓙", "J",
	"𝔍", "J",
	"𝕁", "J",
	"𝕵", "J",
	"𝖩", "J",
	"𝗝", "J",
	"𝘑", "J",
	"𝙅", "J",
	"𝙹", "J",
	"𝐤", "k",
	"𝑘", "k",
	"𝒌", "k",
	"𝓀", "k",
	"𝓴", "k",
	"𝔨", "k",
	"𝕜", "k",
	"𝖐", "k",
	"𝗄", "k",
	"𝗸", "k",
	"𝘬", "k",
	"𝙠", "k",
	"𝚔", "k",
	"ᴋ", "k",
	"ĸ", "k",
	"𝛋", "k",
	"𝛞", "k",
	"𝜅", "k",
	"𝜘", "k",
	"𝜿", "k",
	"𝝒", "k",
	"𝝹", "k",
	"𝞌", "k",
	"𝞳", "k",
	"𝟆", "k",
	"K", "K",
	"Ｋ", "K",
	"𝐊", "K",
	"𝐾", "K",
	"𝑲", "K",
	"𝒦", "K",
	"𝓚", "K",
	"𝔎", "K",
	"𝕂", "K",
	"𝕶", "K",
	"𝖪", "K",
	"𝗞", "K",
	"𝘒", "K",
	"𝙆", "K",
	"𝙺", "K",
	"𝚱", "K",
	"𝛫", "K",
	"𝜥", "K",
	"𝝟", "K",
	"𝞙", "K",
	"|", "l",
	"∣", "l",
	"￨", "l",
	"1", "l",
	"𝟏", "l",
	"𝟙", "l",
	"𝟣", "l",
	"𝟭", "l",
	"𝟷", "l",
	"I", "l",
	"Ｉ", "l",
	"Ⅰ", "l",
	"ℐ", "l",
	"ℑ", "l",
	"𝐈", "l",
	"𝐼", "l",
	"𝑰", "l",
	"𝓘", "l",
	"𝕀", "l",
	"𝕴", "l",
	"𝖨", "l",
	"𝗜", "l",
	"𝘐", "l",
	"𝙄", "l",
	"𝙸", "l",
	"Ɩ", "l",
	"ｌ", "l",
	"ⅼ", "l",
	"ℓ", "l",
	"𝐥", "l",
	"𝑙", "l",
	"𝒍", "l",
	"𝓁", "l",
	"𝓵", "l",
	"𝔩", "l",
	"𝕝", "l",
	"𝖑", "l",
	"𝗅", "l",
	"𝗹", "l",
	"𝘭", "l",
	"𝙡", "l",
	"𝚕", "l",
	"ǀ", "l",
	"𝚰", "l",
	"𝛪", "l",
	"𝜤", "l",
	"𝝞", "l",
	"𝞘", "l",
	"Ⅼ", "L",
	"ℒ", "L",
	"𝐋", "L",
	"𝐿", "L",
	"𝑳", "L",
	"𝓛", "L",
	"𝔏", "L",
	"𝕃", "L",
	"𝕷", "L",
	"𝖫", "L",
	"𝗟", "L",
	"𝘓", "L",
	"𝙇", "L",
	"𝙻", "L",
	"Ｍ", "M",
	"Ⅿ", "M",
	"ℳ", "M",
	"𝐌", "M",
	"𝑀", "M",
	"𝑴", "M",
	"𝓜", "M",
	"𝔐", "M",
	"𝕄", "M",
	"𝕸", "M",
	"𝖬", "M",
	"𝗠", "M",
	"𝘔", "M",
	"𝙈", "M",
	"𝙼", "M",
	"𝚳", "M",
	"𝛭", "M",
	"𝜧", "M",
	"𝝡", "M",
	"𝞛", "M",
	"𝐧", "n",
	"𝑛", "n",
	"𝒏", "n",
	"𝓃", "n",
	"𝓷", "n",
	"𝔫", "n",
	"𝕟", "n",
	"𝖓", "n",
	"𝗇", "n",
	"𝗻", "n",
	"𝘯", "n",
	"𝙣", "n",
	"𝚗", "n",
	"ℼ", "n",
	"𝛑", "n",
	"𝛡", "n",
	"𝜋", "n",
	"𝜛", "n",
	"𝝅", "n",
	"𝝕", "n",
	"𝝿", "n",
	"𝞏", "n",
	"𝞹", "n",
	"𝟉", "n",
	"Ｎ", "N",
	"ℕ", "N",
	"𝐍", "N",
	"𝑁", "N",
	"𝑵", "N",
	"𝒩", "N",
	"𝓝", "N",
	"𝔑", "N",
	"𝕹", "N",
	"𝖭", "N",
	"𝗡", "N",
	"𝘕", "N",
	"𝙉", "N",
	"𝙽", "N",
	"𝚴", "N",
	"𝛮", "N",
	"𝜨", "N",
	"𝝢", "N",
	"𝞜", "N",
	"‎٥‎", "o",
	"ｏ", "o",
	"ℴ", "o",
	"𝐨", "o",
	"𝑜", "o",
	"𝒐", "o",
	"𝓸", "o",
	"𝔬", "o",
	"𝕠", "o",
	"𝖔", "o",
	"𝗈", "o",
	"𝗼", "o",
	"𝘰", "o",
	"𝙤", "o",
	"𝚘", "o",
	"ᴏ", "o",
	"ᴑ", "o",
	"𝛐", "o",
	"𝜊", "o",
	"𝝄", "o",
	"𝝾", "o",
	"𝞸", "o",
	"𝛔", "o",
	"𝜎", "o",
	"𝝈", "o",
	"𝞂", "o",
	"𝞼", "o",
	"0", "O",
	"𝟎", "O",
	"𝟘", "O",
	"𝟢", "O",
	"𝟬", "O",
	"𝟶", "O",
	"Ｏ", "O",
	"𝐎", "O",
	"𝑂", "O",
	"𝑶", "O",
	"𝒪", "O",
	"𝓞", "O",
	"𝔒", "O",
	"𝕆", "O",
	"𝕺", "O",
	"𝖮", "O",
	"𝗢", "O",
	"𝘖", "O",
	"𝙊", "O",
	"𝙾", "O",
	"𝚶", "O",
	"𝛰", "O",
	"𝜪", "O",
	"𝝤", "O",
	"𝞞", "O",
	"⍴", "p",
	"ｐ", "p",
	"𝐩", "p",
	"𝑝", "p",
	"𝒑", "p",
	"𝓅", "p",
	"𝓹", "p",
	"𝔭", "p",
	"𝕡", "p",
	"𝖕", "p",
	"𝗉", "p",
	"𝗽", "p",
	"𝘱", "p",
	"𝙥", "p",
	"𝚙", "p",
	"𝛒", "p",
	"𝛠", "p",
	"𝜌", "p",
	"𝜚", "p",
	"𝝆", "p",
	"𝝔", "p",
	"𝞀", "p",
	"𝞎", "p",
	"𝞺", "p",
	"𝟈", "p",
	"Ｐ", "P",
	"ℙ", "P",
	"𝐏", "P",
	"𝑃", "P",
	"𝑷", "P",
	"𝒫", "P",
	"𝓟", "P",
	"𝔓", "P",
	"𝕻", "P",
	"𝖯", "P",
	"𝗣", "P",
	"𝘗", "P",
	"𝙋", "P",
	"𝙿", "P",
	"𝚸", "P",
	"𝛲", "P",
	"𝜬", "P",
	"𝝦", "P",
	"𝞠", "P",
	"𝐪", "q",
	"𝑞", "q",
	"𝒒", "q",
	"𝓆", "q",
	"𝓺", "q",
	"𝔮", "q",
	"𝕢", "q",
	"𝖖", "q",
	"𝗊", "q",
	"𝗾", "q",
	"𝘲", "q",
	"𝙦", "q",
	"𝚚", "q",
	"ℚ", "Q",
	"𝐐", "Q",
	"𝑄", "Q",
	"𝑸", "Q",
	"𝒬", "Q",
	"𝓠", "Q",
	"𝔔", "Q",
	"𝕼", "Q",
	"𝖰", "Q",
	"𝗤", "Q",
	"𝘘", "Q",
	"𝙌", "Q",
	"𝚀", "Q",
	"𝐫", "r",
	"𝑟", "r",
	"𝒓", "r",
	"𝓇", "r",
	"𝓻", "r",
	"𝔯", "r",
	"𝕣", "r",
	"𝖗", "r",
	"𝗋", "r",
	"𝗿", "r",
	"𝘳", "r",
	"𝙧", "r",
	"𝚛", "r",
	"ℛ", "R",
	"ℜ", "R",
	"ℝ", "R",
	"𝐑", "R",
	"𝑅", "R",
	"𝑹", "R",
	"𝓡", "R",
	"𝕽", "R",
	"𝖱", "R",
	"𝗥", "R",
	"𝘙", "R",
	"𝙍", "R",
	"𝚁", "R",
	"Ʀ", "R",
	"ｓ", "s",
	"𝐬", "s",
	"𝑠", "s",
	"𝒔", "s",
	"𝓈", "s",
	"𝓼", "s",
	"𝔰", "s",
	"𝕤", "s",
	"𝖘", "s",
	"𝗌", "s",
	"𝘀", "s",
	"𝘴", "s",
	"𝙨", "s",
	"𝚜", "s",
	"ꜱ", "s",
	"ƽ", "s",
	"Ｓ", "S",
	"𝐒", "S",
	"𝑆", "S",
	"𝑺", "S",
	"𝒮", "S",
	"𝓢", "S",
	"𝔖", "S",
	"𝕊", "S",
	"𝕾", "S",
	"𝖲", "S",
	"𝗦", "S",
	"𝘚", "S",
	"𝙎", "S",
	"𝚂", "S",
	"𝐭", "t",
	"𝑡", "t",
	"𝒕", "t",
	"𝓉", "t",
	"𝓽", "t",
	"𝔱", "t",
	"𝕥", "t",
	"𝖙", "t",
	"𝗍", "t",
	"𝘁", "t",
	"𝘵", "t",
	"𝙩", "t",
	"𝚝", "t",
	"ᴛ", "t",
	"𝛕", "t",
	"𝜏", "t",
	"𝝉", "t",
	"𝞃", "t",
	"𝞽", "t",
	"⟙", "T",
	"Ｔ", "T",
	"𝐓", "T",
	"𝑇", "T",
	"𝑻", "T",
	"𝒯", "T",
	"𝓣", "T",
	"𝔗", "T",
	"𝕋", "T",
	"𝕿", "T",
	"𝖳", "T",
	"𝗧", "T",
	"𝘛", "T",
	"𝙏", "T",
	"𝚃", "T",
	"𝚻", "T",
	"𝛵", "T",
	"𝜯", "T",
	"𝝩", "T",
	"𝞣", "T",
	"𝐮", "u",
	"𝑢", "u",
	"𝒖", "u",
	"𝓊", "u",
	"𝓾", "u",
	"𝔲", "u",
	"𝕦", "u",
	"𝖚", "u",
	"𝗎", "u",
	"𝘂", "u",
	"𝘶", "u",
	"𝙪", "u",
	"𝚞", "u",
	"ᴜ", "u",
	"ʋ", "u",
	"𝛖", "u",
	"𝜐", "u",
	"𝝊", "u",
	"𝞄", "u",
	"𝞾", "u",
	"𝐔", "U",
	"𝑈", "U",
	"𝑼", "U",
	"𝒰", "U",
	"𝓤", "U",
	"𝔘", "U",
	"𝕌", "U",
	"𝖀", "U",
	"𝖴", "U",
	"𝗨", "U",
	"𝘜", "U",
	"𝙐", "U",
	"𝚄", "U",
	"∨", "v",
	"⋁", "v",
	"ｖ", "v",
	"ⅴ", "v",
	"𝐯", "v",
	"𝑣", "v",
	"𝒗", "v",
	"𝓋", "v",
	"𝓿", "v",
	"𝔳", "v",
	"𝕧", "v",
	"𝖛", "v",
	"𝗏", "v",
	"𝘃", "v",
	"𝘷", "v",
	"𝙫", "v",
	"𝚟", "v",
	"ᴠ", "v",
	"𝛎", "v",
	"𝜈", "v",
	"𝝂", "v",
	"𝝼", "v",
	"𝞶", "v",
	"Ⅴ", "V",
	"𝐕", "V",
	"𝑉", "V",
	"𝑽", "V",
	"𝒱", "V",
	"𝓥", "V",
	"𝔙", "V",
	"𝕍", "V",
	"𝖁", "V",
	"𝖵", "V",
	"𝗩", "V",
	"𝘝", "V",
	"𝙑", "V",
	"𝚅", "V",
	"𝐖", "W",
	"𝑊", "W",
	"𝑾", "W",
	"𝒲", "W",
	"𝓦", "W",
	"𝔚", "W",
	"𝕎", "W",
	"𝖂", "W",
	"𝖶", "W",
	"𝗪", "W",
	"𝘞", "W",
	"𝙒", "W",
	"𝚆", "W",
	"×", "x",
	"╳", "x",
	"⤫", "x",
	"⤬", "x",
	"⨯", "x",
	"ｘ", "x",
	"ⅹ", "x",
	"𝐱", "x",
	"𝑥", "x",
	"𝒙", "x",
	"𝓍", "x",
	"𝔁", "x",
	"𝔵", "x",
	"𝕩", "x",
	"𝖝", "x",
	"𝗑", "x",
	"𝘅", "x",
	"𝘹", "x",
	"𝙭", "x",
	"𝚡", "x",
	"Ｘ", "X",
	"Ⅹ", "X",
	"𝐗", "X",
	"𝑋", "X",
	"𝑿", "X",
	"𝒳", "X",
	"𝓧", "X",
	"𝔛", "X",
	"𝕏", "X",
	"𝖃", "X",
	"𝖷", "X",
	"𝗫", "X",
	"𝘟", "X",
	"𝙓", "X",
	"𝚇", "X",
	"𝚾", "X",
	"𝛸", "X",
	"𝜲", "X",
	"𝝬", "X",
	"𝞦", "X",
	"ᶌ", "y",
	"ｙ", "y",
	"𝐲", "y",
	"𝑦", "y",
	"𝒚", "y",
	"𝓎", "y",
	"𝔂", "y",
	"𝔶", "y",
	"𝕪", "y",
	"𝖞", "y",
	"𝗒", "y",
	"𝘆", "y",
	"𝘺", "y",
	"𝙮", "y",
	"𝚢", "y",
	"ʏ", "y",
	"ỿ", "y",
	"ℽ", "y",
	"𝛄", "y",
	"𝛾", "y",
	"𝜸", "y",
	"𝝲", "y",
	"𝞬", "y",
	"Ｙ", "Y",
	"𝐘", "Y",
	"𝑌", "Y",
	"𝒀", "Y",
	"𝒴", "Y",
	"𝓨", "Y",
	"𝔜", "Y",
	"𝕐", "Y",
	"𝖄", "Y",
	"𝖸", "Y",
	"𝗬", "Y",
	"𝘠", "Y",
	"𝙔", "Y",
	"𝚈", "Y",
	"𝚼", "Y",
	"𝛶", "Y",
	"𝜰", "Y",
	"𝝪", "Y",
	"𝞤", "Y",
	"𝐳", "z",
	"𝑧", "z",
	"𝒛", "z",
	"𝓏", "z",
	"𝔃", "z",
	"𝔷", "z",
	"𝕫", "z",
	"𝖟", "z",
	"𝗓", "z",
	"𝘇", "z",
	"𝘻", "z",
	"𝙯", "z",
	"𝚣", "z",
	"ᴢ", "z",
	"Ｚ", "Z",
	"ℤ", "Z",
	"ℨ", "Z",
	"𝐙", "Z",
	"𝑍", "Z",
	"𝒁", "Z",
	"𝒵", "Z",
	"𝓩", "Z",
	"𝖅", "Z",
	"𝖹", "Z",
	"𝗭", "Z",
	"𝘡", "Z",
	"𝙕", "Z",
	"𝚉", "Z",
	"𝚭", "Z",
	"𝛧", "Z",
	"𝜡", "Z",
	"𝝛", "Z",
	"𝞕", "Z",
	" ", " ",
	" ", " ",
	" ", " ",
	" ", " ",
	" ", " ",
	" ", " ",
	" ", " ",
	" ", " ",
	" ", " ",
	" ", " ",
	" ", " ",
	" ", " ",
	" ", " ",
	" ", " ",
	" ", " ",
	"⍺", "a",
	"ａ", "a",
	"𝐚", "a",
	"𝑎", "a",
	"𝒂", "a",
	"𝒶", "a",
	"𝓪", "a",
	"𝔞", "a",
	"𝕒", "a",
	"𝖆", "a",
	"𝖺", "a",
	"𝗮", "a",
	"𝘢", "a",
	"𝙖", "a",
	"𝚊", "a",
	"ɑ", "a",
	"α", "a",
	"𝛂", "a",
	"𝛼", "a",
	"𝜶", "a",
	"𝝰", "a",
	"𝞪", "a",
	"а", "a",
	"𝐛", "b",
	"𝑏", "b",
	"𝒃", "b",
	"𝒷", "b",
	"𝓫", "b",
	"𝔟", "b",
	"𝕓", "b",
	"𝖇", "b",
	"𝖻", "b",
	"𝗯", "b",
	"𝘣", "b",
	"𝙗", "b",
	"𝚋", "b",
	"Ꮟ", "b",
	"ᖯ", "b",
	"ｃ", "c",
	"ⅽ", "c",
	"𝐜", "c",
	"𝑐", "c",
	"𝒄", "c",
	"𝒸", "c",
	"𝓬", "c",
	"𝔠", "c",
	"𝕔", "c",
	"𝖈", "c",
	"𝖼", "c",
	"𝗰", "c",
	"𝘤", "c",
	"𝙘", "c",
	"𝚌", "c",
	"ᴄ", "c",
	"ϲ", "c",
	"ⲥ", "c",
	"с", "c",
	"ⅾ", "d",
	"ⅆ", "d",
	"𝐝", "d",
	"𝑑", "d",
	"𝒅", "d",
	"𝒹", "d",
	"𝓭", "d",
	"𝔡", "d",
	"𝕕", "d",
	"𝖉", "d",
	"𝖽", "d",
	"𝗱", "d",
	"𝘥", "d",
	"𝙙", "d",
	"𝚍", "d",
	"ԁ", "d",
	"Ꮷ", "d",
	"ᑯ", "d",
	"ꓒ", "d",
	"℮", "e",
	"ｅ", "e",
	"ℯ", "e",
	"ⅇ", "e",
	"𝐞", "e",
	"𝑒", "e",
	"𝒆", "e",
	"𝓮", "e",
	"𝔢", "e",
	"𝕖", "e",
	"𝖊", "e",
	"𝖾", "e",
	"𝗲", "e",
	"𝘦", "e",
	"𝙚", "e",
	"𝚎", "e",
	"е", "e",
	"ҽ", "e",
	"𝐟", "f",
	"𝑓", "f",
	"𝒇", "f",
	"𝒻", "f",
	"𝓯", "f",
	"𝔣", "f",
	"𝕗", "f",
	"𝖋", "f",
	"𝖿", "f",
	"𝗳", "f",
	"𝘧", "f",
	"𝙛", "f",
	"𝚏", "f",
	"ẝ", "f",
	"ք", "f",
	"ｇ", "g",
	"ℊ", "g",
	"𝐠", "g",
	"𝑔", "g",
	"𝒈", "g",
	"𝓰", "g",
	"𝔤", "g",
	"𝕘", "g",
	"𝖌", "g",
	"𝗀", "g",
	"𝗴", "g",
	"𝘨", "g",
	"𝙜", "g",
	"𝚐", "g",
	"ɡ", "g",
	"ᶃ", "g",
	"ƍ", "g",
	"ց", "g",
	"ｈ", "h",
	"ℎ", "h",
	"𝐡", "h",
	"𝒉", "h",
	"𝒽", "h",
	"𝓱", "h",
	"𝔥", "h",
	"𝕙", "h",
	"𝖍", "h",
	"𝗁", "h",
	"𝗵", "h",
	"𝘩", "h",
	"𝙝", "h",
	"𝚑", "h",
	"һ", "h",
	"հ", "h",
	"Ꮒ", "h",
	"˛", "i",
	"⍳", "i",
	"ｉ", "i",
	"ⅰ", "i",
	"ℹ", "i",
	"ⅈ", "i",
	"𝐢", "i",
	"𝑖", "i",
	"𝒊", "i",
	"𝒾", "i",
	"𝓲", "i",
	"𝔦", "i",
	"𝕚", "i",
	"𝖎", "i",
	"𝗂", "i",
	"𝗶", "i",
	"𝘪", "i",
	"𝙞", "i",
	"𝚒", "i",
	"ı", "i",
	"𝚤", "i",
	"ɪ", "i",
	"ɩ", "i",
	"ι", "i",
	"ͺ", "i",
	"𝛊", "i",
	"𝜄", "i",
	"𝜾", "i",
	"𝝸", "i",
	"𝞲", "i",
	"і", "i",
	"ӏ", "i",
	"Ꭵ", "i",
	"ｊ", "j",
	"ⅉ", "j",
	"𝐣", "j",
	"𝑗", "j",
	"𝒋", "j",
	"𝒿", "j",
	"𝓳", "j",
	"𝔧", "j",
	"𝕛", "j",
	"𝖏", "j",
	"𝗃", "j",
	"𝗷", "j",
	"𝘫", "j",
	"𝙟", "j",
	"𝚓", "j",
	"ϳ", "j",
	"ј", "j",
	"𝐤", "k",
	"𝑘", "k",
	"𝒌", "k",
	"𝓀", "k",
	"𝓴", "k",
	"𝔨", "k",
	"𝕜", "k",
	"𝖐", "k",
	"𝗄", "k",
	"𝗸", "k",
	"𝘬", "k",
	"𝙠", "k",
	"𝚔", "k",
	"ᴋ", "k",
	"ĸ", "k",
	"κ", "k",
	"𝛋", "k",
	"𝛞", "k",
	"𝜅", "k",
	"𝜘", "k",
	"𝜿", "k",
	"𝝒", "k",
	"𝝹", "k",
	"𝞌", "k",
	"𝞳", "k",
	"𝟆", "k",
	"ⲕ", "k",
	"к", "k",
	"|", "l",
	"∣", "l",
	"￨", "l",
	"1", "l",
	"𝟏", "l",
	"𝟙", "l",
	"𝟣", "l",
	"𝟭", "l",
	"𝟷", "l",
	"ℐ", "l",
	"ℑ", "l",
	"𝐈", "l",
	"𝐼", "l",
	"𝑰", "l",
	"𝓘", "l",
	"𝕀", "l",
	"𝕴", "l",
	"𝖨", "l",
	"𝗜", "l",
	"𝘐", "l",
	"𝙄", "l",
	"𝙸", "l",
	"ｌ", "l",
	"ⅼ", "l",
	"ℓ", "l",
	"𝐥", "l",
	"𝑙", "l",
	"𝒍", "l",
	"𝓁", "l",
	"𝓵", "l",
	"𝔩", "l",
	"𝕝", "l",
	"𝖑", "l",
	"𝗅", "l",
	"𝗹", "l",
	"𝘭", "l",
	"𝙡", "l",
	"𝚕", "l",
	"ǀ", "l",
	"𝚰", "l",
	"𝛪", "l",
	"𝜤", "l",
	"𝝞", "l",
	"𝞘", "l",
	"‎ו‎", "l",
	"‎ן‎", "l",
	"‎ߊ‎", "l",
	"ⵏ", "l",
	"ꓲ", "l",
	"𝐧", "n",
	"𝑛", "n",
	"𝒏", "n",
	"𝓃", "n",
	"𝓷", "n",
	"𝔫", "n",
	"𝕟", "n",
	"𝖓", "n",
	"𝗇", "n",
	"𝗻", "n",
	"𝘯", "n",
	"𝙣", "n",
	"𝚗", "n",
	"π", "n",
	"ℼ", "n",
	"𝛑", "n",
	"𝛡", "n",
	"𝜋", "n",
	"𝜛", "n",
	"𝝅", "n",
	"𝝕", "n",
	"𝝿", "n",
	"𝞏", "n",
	"𝞹", "n",
	"𝟉", "n",
	"ᴨ", "n",
	"п", "n",
	"ո", "n",
	"ռ", "n",
	"ం", "o",
	"ಂ", "o",
	"ം", "o",
	"ං", "o",
	"०", "o",
	"੦", "o",
	"૦", "o",
	"௦", "o",
	"౦", "o",
	"೦", "o",
	"൦", "o",
	"๐", "o",
	"໐", "o",
	"၀", "o",
	"‎٥‎", "o",
	"ｏ", "o",
	"ℴ", "o",
	"𝐨", "o",
	"𝑜", "o",
	"𝒐", "o",
	"𝓸", "o",
	"𝔬", "o",
	"𝕠", "o",
	"𝖔", "o",
	"𝗈", "o",
	"𝗼", "o",
	"𝘰", "o",
	"𝙤", "o",
	"𝚘", "o",
	"ᴏ", "o",
	"ᴑ", "o",
	"ο", "o",
	"𝛐", "o",
	"𝜊", "o",
	"𝝄", "o",
	"𝝾", "o",
	"𝞸", "o",
	"σ", "o",
	"𝛔", "o",
	"𝜎", "o",
	"𝝈", "o",
	"𝞂", "o",
	"𝞼", "o",
	"ⲟ", "o",
	"о", "o",
	"օ", "o",
	"‎ס‎", "o",
	"ဝ", "o",
	"⍴", "p",
	"ｐ", "p",
	"𝐩", "p",
	"𝑝", "p",
	"𝒑", "p",
	"𝓅", "p",
	"𝓹", "p",
	"𝔭", "p",
	"𝕡", "p",
	"𝖕", "p",
	"𝗉", "p",
	"𝗽", "p",
	"𝘱", "p",
	"𝙥", "p",
	"𝚙", "p",
	"ρ", "p",
	"𝛒", "p",
	"𝛠", "p",
	"𝜌", "p",
	"𝜚", "p",
	"𝝆", "p",
	"𝝔", "p",
	"𝞀", "p",
	"𝞎", "p",
	"𝞺", "p",
	"𝟈", "p",
	"ⲣ", "p",
	"р", "p",
	"𝐪", "q",
	"𝑞", "q",
	"𝒒", "q",
	"𝓆", "q",
	"𝓺", "q",
	"𝔮", "q",
	"𝕢", "q",
	"𝖖", "q",
	"𝗊", "q",
	"𝗾", "q",
	"𝘲", "q",
	"𝙦", "q",
	"𝚚", "q",
	"ԛ", "q",
	"գ", "q",
	"զ", "q",
	"𝐫", "r",
	"𝑟", "r",
	"𝒓", "r",
	"𝓇", "r",
	"𝓻", "r",
	"𝔯", "r",
	"𝕣", "r",
	"𝖗", "r",
	"𝗋", "r",
	"𝗿", "r",
	"𝘳", "r",
	"𝙧", "r",
	"𝚛", "r",
	"ᴦ", "r",
	"ⲅ", "r",
	"г", "r",
	"ｓ", "s",
	"𝐬", "s",
	"𝑠", "s",
	"𝒔", "s",
	"𝓈", "s",
	"𝓼", "s",
	"𝔰", "s",
	"𝕤", "s",
	"𝖘", "s",
	"𝗌", "s",
	"𝘀", "s",
	"𝘴", "s",
	"𝙨", "s",
	"𝚜", "s",
	"ꜱ", "s",
	"ƽ", "s",
	"ѕ", "s",
	"𝐭", "t",
	"𝑡", "t",
	"𝒕", "t",
	"𝓉", "t",
	"𝓽", "t",
	"𝔱", "t",
	"𝕥", "t",
	"𝖙", "t",
	"𝗍", "t",
	"𝘁", "t",
	"𝘵", "t",
	"𝙩", "t",
	"𝚝", "t",
	"ᴛ", "t",
	"τ", "t",
	"𝛕", "t",
	"𝜏", "t",
	"𝝉", "t",
	"𝞃", "t",
	"𝞽", "t",
	"т", "t",
	"𝐮", "u",
	"𝑢", "u",
	"𝒖", "u",
	"𝓊", "u",
	"𝓾", "u",
	"𝔲", "u",
	"𝕦", "u",
	"𝖚", "u",
	"𝗎", "u",
	"𝘂", "u",
	"𝘶", "u",
	"𝙪", "u",
	"𝚞", "u",
	"ᴜ", "u",
	"ʋ", "u",
	"υ", "u",
	"𝛖", "u",
	"𝜐", "u",
	"𝝊", "u",
	"𝞄", "u",
	"𝞾", "u",
	"ц", "u",
	"ս", "u",
	"∨", "v",
	"⋁", "v",
	"ｖ", "v",
	"ⅴ", "v",
	"𝐯", "v",
	"𝑣", "v",
	"𝒗", "v",
	"𝓋", "v",
	"𝓿", "v",
	"𝔳", "v",
	"𝕧", "v",
	"𝖛", "v",
	"𝗏", "v",
	"𝘃", "v",
	"𝘷", "v",
	"𝙫", "v",
	"𝚟", "v",
	"ᴠ", "v",
	"ν", "v",
	"𝛎", "v",
	"𝜈", "v",
	"𝝂", "v",
	"𝝼", "v",
	"𝞶", "v",
	"ѵ", "v",
	"‎ט‎", "v",
	"᙮", "x",
	"×", "x",
	"╳", "x",
	"⤫", "x",
	"⤬", "x",
	"⨯", "x",
	"ｘ", "x",
	"ⅹ", "x",
	"𝐱", "x",
	"𝑥", "x",
	"𝒙", "x",
	"𝓍", "x",
	"𝔁", "x",
	"𝔵", "x",
	"𝕩", "x",
	"𝖝", "x",
	"𝗑", "x",
	"𝘅", "x",
	"𝘹", "x",
	"𝙭", "x",
	"𝚡", "x",
	"х", "x",
	"ᕁ", "x",
	"ᕽ", "x",
	"ᶌ", "y",
	"ｙ", "y",
	"𝐲", "y",
	"𝑦", "y",
	"𝒚", "y",
	"𝓎", "y",
	"𝔂", "y",
	"𝔶", "y",
	"𝕪", "y",
	"𝖞", "y",
	"𝗒", "y",
	"𝘆", "y",
	"𝘺", "y",
	"𝙮", "y",
	"𝚢", "y",
	"ʏ", "y",
	"ỿ", "y",
	"γ", "y",
	"ℽ", "y",
	"𝛄", "y",
	"𝛾", "y",
	"𝜸", "y",
	"𝝲", "y",
	"𝞬", "y",
	"у", "y",
	"ү", "y",
	"ყ", "y",
	"𝐳", "z",
	"𝑧", "z",
	"𝒛", "z",
	"𝓏", "z",
	"𝔃", "z",
	"𝔷", "z",
	"𝕫", "z",
	"𝖟", "z",
	"𝗓", "z",
	"𝘇", "z",
	"𝘻", "z",
	"𝙯", "z",
	"𝚣", "z",
	"ᴢ", "z",
	" ", " ",
	" ", " ",
	" ", " ",
	" ", " ",
	" ", " ",
	" ", " ",
	" ", " ",
	" ", " ",
	" ", " ",
	" ", " ",
	" ", " ",
	" ", " ",
	" ", " ",
	" ", " ",
	" ", " ",
	"⍺", "a",
	"ａ", "a",
	"𝐚", "a",
	"𝑎", "a",
	"𝒂", "a",
	"𝒶", "a",
	"𝓪", "a",
	"𝔞", "a",
	"𝕒", "a",
	"𝖆", "a",
	"𝖺", "a",
	"𝗮", "a",
	"𝘢", "a",
	"𝙖", "a",
	"𝚊", "a",
	"ɑ", "a",
	"α", "a",
	"𝛂", "a",
	"𝛼", "a",
	"𝜶", "a",
	"𝝰", "a",
	"𝞪", "a",
	"а", "a",
	"Ａ", "A",
	"𝐀", "A",
	"𝐴", "A",
	"𝑨", "A",
	"𝒜", "A",
	"𝓐", "A",
	"𝔄", "A",
	"𝔸", "A",
	"𝕬", "A",
	"𝖠", "A",
	"𝗔", "A",
	"𝘈", "A",
	"𝘼", "A",
	"𝙰", "A",
	"Α", "A",
	"𝚨", "A",
	"𝛢", "A",
	"𝜜", "A",
	"𝝖", "A",
	"𝞐", "A",
	"А", "A",
	"Ꭺ", "A",
	"ᗅ", "A",
	"ꓮ", "A",
	"𝐛", "b",
	"𝑏", "b",
	"𝒃", "b",
	"𝒷", "b",
	"𝓫", "b",
	"𝔟", "b",
	"𝕓", "b",
	"𝖇", "b",
	"𝖻", "b",
	"𝗯", "b",
	"𝘣", "b",
	"𝙗", "b",
	"𝚋", "b",
	"Ƅ", "b",
	"Ь", "b",
	"Ꮟ", "b",
	"ᖯ", "b",
	"Ｂ", "B",
	"ℬ", "B",
	"𝐁", "B",
	"𝐵", "B",
	"𝑩", "B",
	"𝓑", "B",
	"𝔅", "B",
	"𝔹", "B",
	"𝕭", "B",
	"𝖡", "B",
	"𝗕", "B",
	"𝘉", "B",
	"𝘽", "B",
	"𝙱", "B",
	"Β", "B",
	"𝚩", "B",
	"𝛣", "B",
	"𝜝", "B",
	"𝝗", "B",
	"𝞑", "B",
	"В", "B",
	"Ᏼ", "B",
	"ᗷ", "B",
	"ꓐ", "B",
	"ｃ", "c",
	"ⅽ", "c",
	"𝐜", "c",
	"𝑐", "c",
	"𝒄", "c",
	"𝒸", "c",
	"𝓬", "c",
	"𝔠", "c",
	"𝕔", "c",
	"𝖈", "c",
	"𝖼", "c",
	"𝗰", "c",
	"𝘤", "c",
	"𝙘", "c",
	"𝚌", "c",
	"ᴄ", "c",
	"ϲ", "c",
	"ⲥ", "c",
	"с", "c",
	"Ｃ", "C",
	"Ⅽ", "C",
	"ℂ", "C",
	"ℭ", "C",
	"𝐂", "C",
	"𝐶", "C",
	"𝑪", "C",
	"𝒞", "C",
	"𝓒", "C",
	"𝕮", "C",
	"𝖢", "C",
	"𝗖", "C",
	"𝘊", "C",
	"𝘾", "C",
	"𝙲", "C",
	"Ϲ", "C",
	"Ⲥ", "C",
	"С", "C",
	"ௐ", "C",
	"Ꮯ", "C",
	"ꓚ", "C",
	"ⅾ", "d",
	"ⅆ", "d",
	"𝐝", "d",
	"𝑑", "d",
	"𝒅", "d",
	"𝒹", "d",
	"𝓭", "d",
	"𝔡", "d",
	"𝕕", "d",
	"𝖉", "d",
	"𝖽", "d",
	"𝗱", "d",
	"𝘥", "d",
	"𝙙", "d",
	"𝚍", "d",
	"ԁ", "d",
	"Ꮷ", "d",
	"ᑯ", "d",
	"ꓒ", "d",
	"Ⅾ", "D",
	"ⅅ", "D",
	"𝐃", "D",
	"𝐷", "D",
	"𝑫", "D",
	"𝒟", "D",
	"𝓓", "D",
	"𝔇", "D",
	"𝔻", "D",
	"𝕯", "D",
	"𝖣", "D",
	"𝗗", "D",
	"𝘋", "D",
	"𝘿", "D",
	"𝙳", "D",
	"Ꭰ", "D",
	"ᗞ", "D",
	"ᗪ", "D",
	"ꓓ", "D",
	"℮", "e",
	"ｅ", "e",
	"ℯ", "e",
	"ⅇ", "e",
	"𝐞", "e",
	"𝑒", "e",
	"𝒆", "e",
	"𝓮", "e",
	"𝔢", "e",
	"𝕖", "e",
	"𝖊", "e",
	"𝖾", "e",
	"𝗲", "e",
	"𝘦", "e",
	"𝙚", "e",
	"𝚎", "e",
	"е", "e",
	"ҽ", "e",
	"⋿", "E",
	"Ｅ", "E",
	"ℰ", "E",
	"𝐄", "E",
	"𝐸", "E",
	"𝑬", "E",
	"𝓔", "E",
	"𝔈", "E",
	"𝔼", "E",
	"𝕰", "E",
	"𝖤", "E",
	"𝗘", "E",
	"𝘌", "E",
	"𝙀", "E",
	"𝙴", "E",
	"Ε", "E",
	"𝚬", "E",
	"𝛦", "E",
	"𝜠", "E",
	"𝝚", "E",
	"𝞔", "E",
	"Е", "E",
	"ⴹ", "E",
	"Ꭼ", "E",
	"ꓰ", "E",
	"𝐟", "f",
	"𝑓", "f",
	"𝒇", "f",
	"𝒻", "f",
	"𝓯", "f",
	"𝔣", "f",
	"𝕗", "f",
	"𝖋", "f",
	"𝖿", "f",
	"𝗳", "f",
	"𝘧", "f",
	"𝙛", "f",
	"𝚏", "f",
	"ſ", "f",
	"ẝ", "f",
	"ք", "f",
	"ℱ", "F",
	"𝐅", "F",
	"𝐹", "F",
	"𝑭", "F",
	"𝓕", "F",
	"𝔉", "F",
	"𝔽", "F",
	"𝕱", "F",
	"𝖥", "F",
	"𝗙", "F",
	"𝘍", "F",
	"𝙁", "F",
	"𝙵", "F",
	"Ϝ", "F",
	"𝟊", "F",
	"ᖴ", "F",
	"ꓝ", "F",
	"ｇ", "g",
	"ℊ", "g",
	"𝐠", "g",
	"𝑔", "g",
	"𝒈", "g",
	"𝓰", "g",
	"𝔤", "g",
	"𝕘", "g",
	"𝖌", "g",
	"𝗀", "g",
	"𝗴", "g",
	"𝘨", "g",
	"𝙜", "g",
	"𝚐", "g",
	"ɡ", "g",
	"ᶃ", "g",
	"ƍ", "g",
	"ց", "g",
	"𝐆", "G",
	"𝐺", "G",
	"𝑮", "G",
	"𝒢", "G",
	"𝓖", "G",
	"𝔊", "G",
	"𝔾", "G",
	"𝕲", "G",
	"𝖦", "G",
	"𝗚", "G",
	"𝘎", "G",
	"𝙂", "G",
	"𝙶", "G",
	"Ԍ", "G",
	"Ꮐ", "G",
	"Ᏻ", "G",
	"ꓖ", "G",
	"ｈ", "h",
	"ℎ", "h",
	"𝐡", "h",
	"𝒉", "h",
	"𝒽", "h",
	"𝓱", "h",
	"𝔥", "h",
	"𝕙", "h",
	"𝖍", "h",
	"𝗁", "h",
	"𝗵", "h",
	"𝘩", "h",
	"𝙝", "h",
	"𝚑", "h",
	"һ", "h",
	"հ", "h",
	"Ꮒ", "h",
	"Ｈ", "H",
	"ℋ", "H",
	"ℌ", "H",
	"ℍ", "H",
	"𝐇", "H",
	"𝐻", "H",
	"𝑯", "H",
	"𝓗", "H",
	"𝕳", "H",
	"𝖧", "H",
	"𝗛", "H",
	"𝘏", "H",
	"𝙃", "H",
	"𝙷", "H",
	"Η", "H",
	"𝚮", "H",
	"𝛨", "H",
	"𝜢", "H",
	"𝝜", "H",
	"𝞖", "H",
	"Ⲏ", "H",
	"Н", "H",
	"Ꮋ", "H",
	"ᕼ", "H",
	"ꓧ", "H",
	"˛", "i",
	"⍳", "i",
	"ｉ", "i",
	"ⅰ", "i",
	"ℹ", "i",
	"ⅈ", "i",
	"𝐢", "i",
	"𝑖", "i",
	"𝒊", "i",
	"𝒾", "i",
	"𝓲", "i",
	"𝔦", "i",
	"𝕚", "i",
	"𝖎", "i",
	"𝗂", "i",
	"𝗶", "i",
	"𝘪", "i",
	"𝙞", "i",
	"𝚒", "i",
	"ı", "i",
	"𝚤", "i",
	"ɪ", "i",
	"ɩ", "i",
	"ι", "i",
	"ι", "i",
	"ͺ", "i",
	"𝛊", "i",
	"𝜄", "i",
	"𝜾", "i",
	"𝝸", "i",
	"𝞲", "i",
	"і", "i",
	"ӏ", "i",
	"Ꭵ", "i",
	"ｊ", "j",
	"ⅉ", "j",
	"𝐣", "j",
	"𝑗", "j",
	"𝒋", "j",
	"𝒿", "j",
	"𝓳", "j",
	"𝔧", "j",
	"𝕛", "j",
	"𝖏", "j",
	"𝗃", "j",
	"𝗷", "j",
	"𝘫", "j",
	"𝙟", "j",
	"𝚓", "j",
	"ϳ", "j",
	"ј", "j",
	"Ｊ", "J",
	"𝐉", "J",
	"𝐽", "J",
	"𝑱", "J",
	"𝒥", "J",
	"𝓙", "J",
	"𝔍", "J",
	"𝕁", "J",
	"𝕵", "J",
	"𝖩", "J",
	"𝗝", "J",
	"𝘑", "J",
	"𝙅", "J",
	"𝙹", "J",
	"Ј", "J",
	"Ꭻ", "J",
	"ᒍ", "J",
	"ꓙ", "J",
	"𝐤", "k",
	"𝑘", "k",
	"𝒌", "k",
	"𝓀", "k",
	"𝓴", "k",
	"𝔨", "k",
	"𝕜", "k",
	"𝖐", "k",
	"𝗄", "k",
	"𝗸", "k",
	"𝘬", "k",
	"𝙠", "k",
	"𝚔", "k",
	"ᴋ", "k",
	"ĸ", "k",
	"κ", "k",
	"ϰ", "k",
	"𝛋", "k",
	"𝛞", "k",
	"𝜅", "k",
	"𝜘", "k",
	"𝜿", "k",
	"𝝒", "k",
	"𝝹", "k",
	"𝞌", "k",
	"𝞳", "k",
	"𝟆", "k",
	"ⲕ", "k",
	"к", "k",
	"K", "K",
	"Ｋ", "K",
	"𝐊", "K",
	"𝐾", "K",
	"𝑲", "K",
	"𝒦", "K",
	"𝓚", "K",
	"𝔎", "K",
	"𝕂", "K",
	"𝕶", "K",
	"𝖪", "K",
	"𝗞", "K",
	"𝘒", "K",
	"𝙆", "K",
	"𝙺", "K",
	"Κ", "K",
	"𝚱", "K",
	"𝛫", "K",
	"𝜥", "K",
	"𝝟", "K",
	"𝞙", "K",
	"Ⲕ", "K",
	"К", "K",
	"Ꮶ", "K",
	"ꓗ", "K",
	"|", "l",
	"∣", "l",
	"￨", "l",
	"1", "l",
	"𝟏", "l",
	"𝟙", "l",
	"𝟣", "l",
	"𝟭", "l",
	"𝟷", "l",
	"I", "l",
	"Ｉ", "l",
	"Ⅰ", "l",
	"ℐ", "l",
	"ℑ", "l",
	"𝐈", "l",
	"𝐼", "l",
	"𝑰", "l",
	"𝓘", "l",
	"𝕀", "l",
	"𝕴", "l",
	"𝖨", "l",
	"𝗜", "l",
	"𝘐", "l",
	"𝙄", "l",
	"𝙸", "l",
	"Ɩ", "l",
	"ｌ", "l",
	"ⅼ", "l",
	"ℓ", "l",
	"𝐥", "l",
	"𝑙", "l",
	"𝒍", "l",
	"𝓁", "l",
	"𝓵", "l",
	"𝔩", "l",
	"𝕝", "l",
	"𝖑", "l",
	"𝗅", "l",
	"𝗹", "l",
	"𝘭", "l",
	"𝙡", "l",
	"𝚕", "l",
	"ǀ", "l",
	"Ι", "l",
	"𝚰", "l",
	"𝛪", "l",
	"𝜤", "l",
	"𝝞", "l",
	"𝞘", "l",
	"Ⲓ", "l",
	"І", "l",
	"Ӏ", "l",
	"‎ו‎", "l",
	"‎ן‎", "l",
	"‎ߊ‎", "l",
	"ⵏ", "l",
	"ꓲ", "l",
	"Ⅼ", "L",
	"ℒ", "L",
	"𝐋", "L",
	"𝐿", "L",
	"𝑳", "L",
	"𝓛", "L",
	"𝔏", "L",
	"𝕃", "L",
	"𝕷", "L",
	"𝖫", "L",
	"𝗟", "L",
	"𝘓", "L",
	"𝙇", "L",
	"𝙻", "L",
	"Ⳑ", "L",
	"Ꮮ", "L",
	"ᒪ", "L",
	"ꓡ", "L",
	"Ｍ", "M",
	"Ⅿ", "M",
	"ℳ", "M",
	"𝐌", "M",
	"𝑀", "M",
	"𝑴", "M",
	"𝓜", "M",
	"𝔐", "M",
	"𝕄", "M",
	"𝕸", "M",
	"𝖬", "M",
	"𝗠", "M",
	"𝘔", "M",
	"𝙈", "M",
	"𝙼", "M",
	"Μ", "M",
	"𝚳", "M",
	"𝛭", "M",
	"𝜧", "M",
	"𝝡", "M",
	"𝞛", "M",
	"Ϻ", "M",
	"Ⲙ", "M",
	"М", "M",
	"Ꮇ", "M",
	"ᗰ", "M",
	"ꓟ", "M",
	"𝐧", "n",
	"𝑛", "n",
	"𝒏", "n",
	"𝓃", "n",
	"𝓷", "n",
	"𝔫", "n",
	"𝕟", "n",
	"𝖓", "n",
	"𝗇", "n",
	"𝗻", "n",
	"𝘯", "n",
	"𝙣", "n",
	"𝚗", "n",
	"π", "n",
	"ϖ", "n",
	"ℼ", "n",
	"𝛑", "n",
	"𝛡", "n",
	"𝜋", "n",
	"𝜛", "n",
	"𝝅", "n",
	"𝝕", "n",
	"𝝿", "n",
	"𝞏", "n",
	"𝞹", "n",
	"𝟉", "n",
	"ᴨ", "n",
	"п", "n",
	"ո", "n",
	"ռ", "n",
	"Ｎ", "N",
	"ℕ", "N",
	"𝐍", "N",
	"𝑁", "N",
	"𝑵", "N",
	"𝒩", "N",
	"𝓝", "N",
	"𝔑", "N",
	"𝕹", "N",
	"𝖭", "N",
	"𝗡", "N",
	"𝘕", "N",
	"𝙉", "N",
	"𝙽", "N",
	"Ν", "N",
	"𝚴", "N",
	"𝛮", "N",
	"𝜨", "N",
	"𝝢", "N",
	"𝞜", "N",
	"Ⲛ", "N",
	"ꓠ", "N",
	"ం", "o",
	"ಂ", "o",
	"ം", "o",
	"ං", "o",
	"०", "o",
	"੦", "o",
	"૦", "o",
	"௦", "o",
	"౦", "o",
	"೦", "o",
	"൦", "o",
	"๐", "o",
	"໐", "o",
	"၀", "o",
	"‎٥‎", "o",
	"ｏ", "o",
	"ℴ", "o",
	"𝐨", "o",
	"𝑜", "o",
	"𝒐", "o",
	"𝓸", "o",
	"𝔬", "o",
	"𝕠", "o",
	"𝖔", "o",
	"𝗈", "o",
	"𝗼", "o",
	"𝘰", "o",
	"𝙤", "o",
	"𝚘", "o",
	"ᴏ", "o",
	"ᴑ", "o",
	"ο", "o",
	"𝛐", "o",
	"𝜊", "o",
	"𝝄", "o",
	"𝝾", "o",
	"𝞸", "o",
	"σ", "o",
	"𝛔", "o",
	"𝜎", "o",
	"𝝈", "o",
	"𝞂", "o",
	"𝞼", "o",
	"ⲟ", "o",
	"о", "o",
	"օ", "o",
	"‎ס‎", "o",
	"ဝ", "o",
	"0", "O",
	"𝟎", "O",
	"𝟘", "O",
	"𝟢", "O",
	"𝟬", "O",
	"𝟶", "O",
	"‎߀‎", "O",
	"০", "O",
	"୦", "O",
	"〇", "O",
	"Ｏ", "O",
	"𝐎", "O",
	"𝑂", "O",
	"𝑶", "O",
	"𝒪", "O",
	"𝓞", "O",
	"𝔒", "O",
	"𝕆", "O",
	"𝕺", "O",
	"𝖮", "O",
	"𝗢", "O",
	"𝘖", "O",
	"𝙊", "O",
	"𝙾", "O",
	"Ο", "O",
	"𝚶", "O",
	"𝛰", "O",
	"𝜪", "O",
	"𝝤", "O",
	"𝞞", "O",
	"Ⲟ", "O",
	"О", "O",
	"Օ", "O",
	"ⵔ", "O",
	"ଠ", "O",
	"ഠ", "O",
	"ꓳ", "O",
	"⍴", "p",
	"ｐ", "p",
	"𝐩", "p",
	"𝑝", "p",
	"𝒑", "p",
	"𝓅", "p",
	"𝓹", "p",
	"𝔭", "p",
	"𝕡", "p",
	"𝖕", "p",
	"𝗉", "p",
	"𝗽", "p",
	"𝘱", "p",
	"𝙥", "p",
	"𝚙", "p",
	"ρ", "p",
	"ϱ", "p",
	"𝛒", "p",
	"𝛠", "p",
	"𝜌", "p",
	"𝜚", "p",
	"𝝆", "p",
	"𝝔", "p",
	"𝞀", "p",
	"𝞎", "p",
	"𝞺", "p",
	"𝟈", "p",
	"ⲣ", "p",
	"р", "p",
	"Ｐ", "P",
	"ℙ", "P",
	"𝐏", "P",
	"𝑃", "P",
	"𝑷", "P",
	"𝒫", "P",
	"𝓟", "P",
	"𝔓", "P",
	"𝕻", "P",
	"𝖯", "P",
	"𝗣", "P",
	"𝘗", "P",
	"𝙋", "P",
	"𝙿", "P",
	"Ρ", "P",
	"𝚸", "P",
	"𝛲", "P",
	"𝜬", "P",
	"𝝦", "P",
	"𝞠", "P",
	"Ⲣ", "P",
	"Р", "P",
	"Ꮲ", "P",
	"ᑭ", "P",
	"ꓑ", "P",
	"𝐪", "q",
	"𝑞", "q",
	"𝒒", "q",
	"𝓆", "q",
	"𝓺", "q",
	"𝔮", "q",
	"𝕢", "q",
	"𝖖", "q",
	"𝗊", "q",
	"𝗾", "q",
	"𝘲", "q",
	"𝙦", "q",
	"𝚚", "q",
	"ԛ", "q",
	"գ", "q",
	"զ", "q",
	"ℚ", "Q",
	"𝐐", "Q",
	"𝑄", "Q",
	"𝑸", "Q",
	"𝒬", "Q",
	"𝓠", "Q",
	"𝔔", "Q",
	"𝕼", "Q",
	"𝖰", "Q",
	"𝗤", "Q",
	"𝘘", "Q",
	"𝙌", "Q",
	"𝚀", "Q",
	"𝐫", "r",
	"𝑟", "r",
	"𝒓", "r",
	"𝓇", "r",
	"𝓻", "r",
	"𝔯", "r",
	"𝕣", "r",
	"𝖗", "r",
	"𝗋", "r",
	"𝗿", "r",
	"𝘳", "r",
	"𝙧", "r",
	"𝚛", "r",
	"ᴦ", "r",
	"ⲅ", "r",
	"г", "r",
	"ℛ", "R",
	"ℜ", "R",
	"ℝ", "R",
	"𝐑", "R",
	"𝑅", "R",
	"𝑹", "R",
	"𝓡", "R",
	"𝕽", "R",
	"𝖱", "R",
	"𝗥", "R",
	"𝘙", "R",
	"𝙍", "R",
	"𝚁", "R",
	"Ʀ", "R",
	"Ꭱ", "R",
	"Ꮢ", "R",
	"ᖇ", "R",
	"ꓣ", "R",
	"ｓ", "s",
	"𝐬", "s",
	"𝑠", "s",
	"𝒔", "s",
	"𝓈", "s",
	"𝓼", "s",
	"𝔰", "s",
	"𝕤", "s",
	"𝖘", "s",
	"𝗌", "s",
	"𝘀", "s",
	"𝘴", "s",
	"𝙨", "s",
	"𝚜", "s",
	"ꜱ", "s",
	"ƽ", "s",
	"ѕ", "s",
	"Ｓ", "S",
	"𝐒", "S",
	"𝑆", "S",
	"𝑺", "S",
	"𝒮", "S",
	"𝓢", "S",
	"𝔖", "S",
	"𝕊", "S",
	"𝕾", "S",
	"𝖲", "S",
	"𝗦", "S",
	"𝘚", "S",
	"𝙎", "S",
	"𝚂", "S",
	"Ѕ", "S",
	"Տ", "S",
	"Ꮥ", "S",
	"Ꮪ", "S",
	"ꓢ", "S",
	"𝐭", "t",
	"𝑡", "t",
	"𝒕", "t",
	"𝓉", "t",
	"𝓽", "t",
	"𝔱", "t",
	"𝕥", "t",
	"𝖙", "t",
	"𝗍", "t",
	"𝘁", "t",
	"𝘵", "t",
	"𝙩", "t",
	"𝚝", "t",
	"ᴛ", "t",
	"τ", "t",
	"𝛕", "t",
	"𝜏", "t",
	"𝝉", "t",
	"𝞃", "t",
	"𝞽", "t",
	"т", "t",
	"⟙", "T",
	"Ｔ", "T",
	"𝐓", "T",
	"𝑇", "T",
	"𝑻", "T",
	"𝒯", "T",
	"𝓣", "T",
	"𝔗", "T",
	"𝕋", "T",
	"𝕿", "T",
	"𝖳", "T",
	"𝗧", "T",
	"𝘛", "T",
	"𝙏", "T",
	"𝚃", "T",
	"Τ", "T",
	"𝚻", "T",
	"𝛵", "T",
	"𝜯", "T",
	"𝝩", "T",
	"𝞣", "T",
	"Ⲧ", "T",
	"Т", "T",
	"Ꭲ", "T",
	"ꓔ", "T",
	"𝐮", "u",
	"𝑢", "u",
	"𝒖", "u",
	"𝓊", "u",
	"𝓾", "u",
	"𝔲", "u",
	"𝕦", "u",
	"𝖚", "u",
	"𝗎", "u",
	"𝘂", "u",
	"𝘶", "u",
	"𝙪", "u",
	"𝚞", "u",
	"ᴜ", "u",
	"ʋ", "u",
	"υ", "u",
	"𝛖", "u",
	"𝜐", "u",
	"𝝊", "u",
	"𝞄", "u",
	"𝞾", "u",
	"ц", "u",
	"ս", "u",
	"𝐔", "U",
	"𝑈", "U",
	"𝑼", "U",
	"𝒰", "U",
	"𝓤", "U",
	"𝔘", "U",
	"𝕌", "U",
	"𝖀", "U",
	"𝖴", "U",
	"𝗨", "U",
	"𝘜", "U",
	"𝙐", "U",
	"𝚄", "U",
	"Ս", "U",
	"ᑌ", "U",
	"ꓴ", "U",
	"∨", "v",
	"⋁", "v",
	"ｖ", "v",
	"ⅴ", "v",
	"𝐯", "v",
	"𝑣", "v",
	"𝒗", "v",
	"𝓋", "v",
	"𝓿", "v",
	"𝔳", "v",
	"𝕧", "v",
	"𝖛", "v",
	"𝗏", "v",
	"𝘃", "v",
	"𝘷", "v",
	"𝙫", "v",
	"𝚟", "v",
	"ᴠ", "v",
	"ν", "v",
	"𝛎", "v",
	"𝜈", "v",
	"𝝂", "v",
	"𝝼", "v",
	"𝞶", "v",
	"ѵ", "v",
	"‎ט‎", "v",
	"Ⅴ", "V",
	"𝐕", "V",
	"𝑉", "V",
	"𝑽", "V",
	"𝒱", "V",
	"𝓥", "V",
	"𝔙", "V",
	"𝕍", "V",
	"𝖁", "V",
	"𝖵", "V",
	"𝗩", "V",
	"𝘝", "V",
	"𝙑", "V",
	"𝚅", "V",
	"Ѵ", "V",
	"ⴸ", "V",
	"Ꮩ", "V",
	"ᐯ", "V",
	"ꓦ", "V",
	"𝐖", "W",
	"𝑊", "W",
	"𝑾", "W",
	"𝒲", "W",
	"𝓦", "W",
	"𝔚", "W",
	"𝕎", "W",
	"𝖂", "W",
	"𝖶", "W",
	"𝗪", "W",
	"𝘞", "W",
	"𝙒", "W",
	"𝚆", "W",
	"Ԝ", "W",
	"Ꮃ", "W",
	"Ꮤ", "W",
	"ꓪ", "W",
	"᙮", "x",
	"×", "x",
	"╳", "x",
	"⤫", "x",
	"⤬", "x",
	"⨯", "x",
	"ｘ", "x",
	"ⅹ", "x",
	"𝐱", "x",
	"𝑥", "x",
	"𝒙", "x",
	"𝓍", "x",
	"𝔁", "x",
	"𝔵", "x",
	"𝕩", "x",
	"𝖝", "x",
	"𝗑", "x",
	"𝘅", "x",
	"𝘹", "x",
	"𝙭", "x",
	"𝚡", "x",
	"х", "x",
	"ᕁ", "x",
	"ᕽ", "x",
	"᙭", "X",
	"Ｘ", "X",
	"Ⅹ", "X",
	"𝐗", "X",
	"𝑋", "X",
	"𝑿", "X",
	"𝒳", "X",
	"𝓧", "X",
	"𝔛", "X",
	"𝕏", "X",
	"𝖃", "X",
	"𝖷", "X",
	"𝗫", "X",
	"𝘟", "X",
	"𝙓", "X",
	"𝚇", "X",
	"Χ", "X",
	"𝚾", "X",
	"𝛸", "X",
	"𝜲", "X",
	"𝝬", "X",
	"𝞦", "X",
	"Ⲭ", "X",
	"Х", "X",
	"ⵝ", "X",
	"ꓫ", "X",
	"ᶌ", "y",
	"ｙ", "y",
	"𝐲", "y",
	"𝑦", "y",
	"𝒚", "y",
	"𝓎", "y",
	"𝔂", "y",
	"𝔶", "y",
	"𝕪", "y",
	"𝖞", "y",
	"𝗒", "y",
	"𝘆", "y",
	"𝘺", "y",
	"𝙮", "y",
	"𝚢", "y",
	"ʏ", "y",
	"ỿ", "y",
	"γ", "y",
	"ℽ", "y",
	"𝛄", "y",
	"𝛾", "y",
	"𝜸", "y",
	"𝝲", "y",
	"𝞬", "y",
	"у", "y",
	"ү", "y",
	"ყ", "y",
	"Ｙ", "Y",
	"𝐘", "Y",
	"𝑌", "Y",
	"𝒀", "Y",
	"𝒴", "Y",
	"𝓨", "Y",
	"𝔜", "Y",
	"𝕐", "Y",
	"𝖄", "Y",
	"𝖸", "Y",
	"𝗬", "Y",
	"𝘠", "Y",
	"𝙔", "Y",
	"𝚈", "Y",
	"Υ", "Y",
	"ϒ", "Y",
	"𝚼", "Y",
	"𝛶", "Y",
	"𝜰", "Y",
	"𝝪", "Y",
	"𝞤", "Y",
	"Ⲩ", "Y",
	"Ү", "Y",
	"Ꭹ", "Y",
	"Ꮍ", "Y",
	"ꓬ", "Y",
	"𝐳", "z",
	"𝑧", "z",
	"𝒛", "z",
	"𝓏", "z",
	"𝔃", "z",
	"𝔷", "z",
	"𝕫", "z",
	"𝖟", "z",
	"𝗓", "z",
	"𝘇", "z",
	"𝘻", "z",
	"𝙯", "z",
	"𝚣", "z",
	"ᴢ", "z",
	"Ｚ", "Z",
	"ℤ", "Z",
	"ℨ", "Z",
	"𝐙", "Z",
	"𝑍", "Z",
	"𝒁", "Z",
	"𝒵", "Z",
	"𝓩", "Z",
	"𝖅", "Z",
	"𝖹", "Z",
	"𝗭", "Z",
	"𝘡", "Z",
	"𝙕", "Z",
	"𝚉", "Z",
	"Ζ", "Z",
	"𝚭", "Z",
	"𝛧", "Z",
	"𝜡", "Z",
	"𝝛", "Z",
	"𝞕", "Z",
	"Ꮓ", "Z",
	"ꓜ", "Z",
)

var Santize = Santizier.Replace
